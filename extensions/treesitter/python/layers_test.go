package python

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Django app and a FastAPI service, read the way the lanes read them. The
// UI's layers test (archstats-ui, layers.server.test.ts) is built from what
// these assert: the markers with their source, the raw imports, and the refs
// that become edges between layers.

func filename(key string) unit.Marker  { return unit.Marker{Source: unit.SourceFilename, Key: key} }
func dir(key string) unit.Marker       { return unit.Marker{Source: unit.SourcePath, Key: key} }
func super(key string) unit.Marker     { return unit.Marker{Source: unit.SourceSupertype, Key: key} }
func decorated(key string) unit.Marker { return unit.Marker{Source: unit.SourceAnnotation, Key: key} }

func rawImports(t *testing.T, path, src string) []string {
	t.Helper()
	res := createPythonLanguagePack().AnalyzeFileContent(path, []byte(src))
	require.NotNil(t, res)
	var out []string
	for _, s := range res.Snippets {
		if s.Type == file.ImportRaw {
			out = append(out, s.Value)
		}
	}
	return out
}

func TestDjangoModelsCarryTheFilenameAndTheBaseClass(t *testing.T) {
	src := `from django.db import models
from .managers import ProductManager


class Product(models.Model):
    title = models.CharField(max_length=200)
    objects = ProductManager()

    class Meta:
        ordering = ["title"]

    @property
    def label(self):
        return self.title


class Category(models.Model):
    pass
`
	byID := unitsIn(t, "shop/catalogue/models.py", src)

	product := byID["shop/catalogue/models#Product"]
	require.NotNil(t, product)
	assert.Equal(t, unit.KindType, product.Kind)
	assert.Empty(t, product.Owner)
	assert.ElementsMatch(t, []unit.Marker{filename("models"), super("Model")}, product.Markers)
	assert.Contains(t, product.Refs, unit.Ref{Module: "./managers", Name: "ProductManager"})

	// A method's decorator is the method's. It used to land on the next class
	// in the file as well: 209 of oscar's types carried `property`.
	label := byID["shop/catalogue/models#Product.label"]
	require.NotNil(t, label)
	assert.Equal(t, "shop/catalogue/models#Product", label.Owner)
	assert.Contains(t, label.Markers, decorated("property"))
	category := byID["shop/catalogue/models#Category"]
	require.NotNil(t, category)
	assert.ElementsMatch(t, []unit.Marker{filename("models"), super("Model")}, category.Markers)

	assert.ElementsMatch(t, []string{"django.db", ".managers"}, rawImports(t, "shop/catalogue/models.py", src))
}

// A big app splits its models into a package. `models/order.py` carried no
// marker at all, so every model written that way fell out of its lane.
func TestDjangoModelsPackageIsEvidenceToo(t *testing.T) {
	src := `from django.db import models
from shop.catalogue.models import Product


class Order(models.Model):
    product = models.ForeignKey(Product, on_delete=models.CASCADE)
`
	byID := unitsIn(t, "shop/orders/models/order.py", src)
	order := byID["shop/orders/models/order#Order"]
	require.NotNil(t, order)
	assert.ElementsMatch(t, []unit.Marker{dir("models"), super("Model")}, order.Markers)
	assert.Contains(t, order.Refs, unit.Ref{Module: "shop.catalogue.models", Name: "Product"})

	abstract := unitsIn(t, "shop/catalogue/abstract_models.py", "from django.db import models\n\nclass AbstractProduct(models.Model):\n    class Meta:\n        abstract = True\n")
	assert.ElementsMatch(t, []unit.Marker{filename("abstract_models"), super("Model")}, abstract["shop/catalogue/abstract_models#AbstractProduct"].Markers)
}

// A migration is generated code that names its models by string. It is
// marked by the directory it lives in, which is the only thing about it
// that says what it is; oscar has 182 of them.
func TestMigrationsAreMarkedByTheirDirectory(t *testing.T) {
	src := `from django.db import migrations, models


class Migration(migrations.Migration):
    initial = True
    dependencies = []
    operations = [migrations.CreateModel(name="Product", fields=[])]
`
	byID := unitsIn(t, "shop/catalogue/migrations/0001_initial.py", src)
	m := byID["shop/catalogue/migrations/0001_initial#Migration"]
	require.NotNil(t, m)
	assert.ElementsMatch(t, []unit.Marker{dir("migrations"), super("Migration")}, m.Markers)
}

func TestDjangoViewsFunctionsAndClasses(t *testing.T) {
	src := `from django.views.generic import ListView
from django.shortcuts import render
from .models import Product
from .forms import ProductForm


class ProductListView(ListView):
    model = Product


def product_create(request):
    form = ProductForm(request.POST)
    return render(request, "x.html", {"form": form})
`
	byID := unitsIn(t, "shop/catalogue/views.py", src)

	view := byID["shop/catalogue/views#ProductListView"]
	require.NotNil(t, view)
	assert.Equal(t, unit.KindType, view.Kind)
	assert.ElementsMatch(t, []unit.Marker{filename("views"), super("ListView")}, view.Markers)
	assert.Contains(t, view.Refs, unit.Ref{Module: "./models", Name: "Product"})
	assert.Contains(t, view.Refs, unit.Ref{Module: "django.views.generic", Name: "ListView"})

	fn := byID["shop/catalogue/views#product_create"]
	require.NotNil(t, fn)
	assert.Equal(t, unit.KindFunction, fn.Kind)
	assert.Empty(t, fn.Owner)
	assert.ElementsMatch(t, []unit.Marker{filename("views")}, fn.Markers)
	assert.Contains(t, fn.Refs, unit.Ref{Module: "./forms", Name: "ProductForm"})
	assert.Contains(t, fn.Refs, unit.Ref{Module: "django.shortcuts", Name: "render"})

	assert.ElementsMatch(t, []string{"django.views.generic", "django.shortcuts", ".models", ".forms"}, rawImports(t, "shop/catalogue/views.py", src))
}

// A form's or serializer's model is named inside its Meta, so the reference
// belongs to the nested class and reaches the form through its owner. The
// UI rolls members up to their owners before it reads edges.
func TestFormsAndSerializersReachTheirModelThroughMeta(t *testing.T) {
	forms := unitsIn(t, "shop/catalogue/forms.py", `from django import forms
from .models import Product


class ProductForm(forms.ModelForm):
    class Meta:
        model = Product
`)
	form := forms["shop/catalogue/forms#ProductForm"]
	require.NotNil(t, form)
	assert.ElementsMatch(t, []unit.Marker{filename("forms"), super("ModelForm")}, form.Markers)
	assert.NotContains(t, form.Refs, unit.Ref{Module: "./models", Name: "Product"})
	meta := forms["shop/catalogue/forms#ProductForm.Meta"]
	require.NotNil(t, meta)
	assert.Equal(t, "shop/catalogue/forms#ProductForm", meta.Owner)
	assert.Equal(t, []unit.Ref{{Module: "./models", Name: "Product"}}, meta.Refs)

	serializers := unitsIn(t, "shop/catalogue/serializers.py", `from rest_framework import serializers
from .models import Product


class ProductSerializer(serializers.ModelSerializer):
    class Meta:
        model = Product
        fields = "__all__"
`)
	s := serializers["shop/catalogue/serializers#ProductSerializer"]
	require.NotNil(t, s)
	assert.ElementsMatch(t, []unit.Marker{filename("serializers"), super("ModelSerializer")}, s.Markers)
	assert.Equal(t, "shop/catalogue/serializers#ProductSerializer", serializers["shop/catalogue/serializers#ProductSerializer.Meta"].Owner)
}

// A DRF viewset in a file Django gives no name is known by its base class,
// and a decorated action inside it marks the action and nothing after it.
func TestViewsetsInAnUnnamedFileAreKnownByTheirBase(t *testing.T) {
	src := `from rest_framework import viewsets
from rest_framework.decorators import action
from .models import Product
from .serializers import ProductSerializer


class ProductViewSet(viewsets.ModelViewSet):
    queryset = Product.objects.all()
    serializer_class = ProductSerializer

    @action(detail=True)
    def publish(self, request, pk=None):
        pass


class Unrelated:
    pass
`
	byID := unitsIn(t, "shop/catalogue/api.py", src)
	vs := byID["shop/catalogue/api#ProductViewSet"]
	require.NotNil(t, vs)
	assert.Equal(t, []unit.Marker{super("ModelViewSet")}, vs.Markers)
	assert.Contains(t, vs.Refs, unit.Ref{Module: "./models", Name: "Product"})
	assert.Contains(t, vs.Refs, unit.Ref{Module: "./serializers", Name: "ProductSerializer"})
	assert.Contains(t, byID["shop/catalogue/api#ProductViewSet.publish"].Markers, decorated("action"))
	assert.Empty(t, byID["shop/catalogue/api#Unrelated"].Markers, "a method's decorator landed on the next class")
}

func TestAdminSignalsAppsAndUrls(t *testing.T) {
	admin := unitsIn(t, "shop/catalogue/admin.py", `from django.contrib import admin
from .models import Product


@admin.register(Product)
class ProductAdmin(admin.ModelAdmin):
    list_display = ["title"]
`)
	a := admin["shop/catalogue/admin#ProductAdmin"]
	require.NotNil(t, a)
	assert.ElementsMatch(t, []unit.Marker{filename("admin"), super("ModelAdmin"), decorated("register")}, a.Markers)
	assert.Contains(t, a.Refs, unit.Ref{Module: "./models", Name: "Product"})

	signals := unitsIn(t, "shop/catalogue/signals.py", `from django.db.models.signals import post_save
from django.dispatch import receiver
from .models import Product


@receiver(post_save, sender=Product)
def on_product_saved(sender, instance, **kwargs):
    pass
`)
	r := signals["shop/catalogue/signals#on_product_saved"]
	require.NotNil(t, r)
	assert.Equal(t, unit.KindFunction, r.Kind)
	assert.ElementsMatch(t, []unit.Marker{filename("signals"), decorated("receiver")}, r.Markers)
	assert.Contains(t, r.Refs, unit.Ref{Module: "./models", Name: "Product"})

	apps := unitsIn(t, "shop/catalogue/apps.py", `from django.apps import AppConfig


class CatalogueConfig(AppConfig):
    name = "shop.catalogue"

    def ready(self):
        from . import signals
`)
	assert.ElementsMatch(t, []unit.Marker{filename("apps"), super("AppConfig")}, apps["shop/catalogue/apps#CatalogueConfig"].Markers)

	// urls.py declares nothing; what it names belongs to the module.
	urls := unitsIn(t, "shop/catalogue/urls.py", `from django.urls import path
from . import views

urlpatterns = [path("", views.ProductListView.as_view())]
`)
	mod := urls["shop/catalogue/urls#"]
	require.NotNil(t, mod)
	assert.Equal(t, unit.KindModule, mod.Kind)
	assert.Contains(t, mod.Refs, unit.Ref{Module: ".", Name: "views"})
}

// The planted violation: a model that reaches up into a view. The engine
// records the reference like any other; the UI's layer order is what reads
// it as running back up.
func TestAModelImportingAViewIsARefLikeAnyOther(t *testing.T) {
	byID := unitsIn(t, "shop/reviews/models.py", `from django.db import models
from shop.catalogue.views import ProductListView


class Review(models.Model):
    view = ProductListView
`)
	assert.Contains(t, byID["shop/reviews/models#Review"].Refs, unit.Ref{Module: "shop.catalogue.views", Name: "ProductListView"})
}

func TestFastAPIRoutesAreDecoratedFunctions(t *testing.T) {
	src := `from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session
from ..schemas import UserOut
from ..repositories import UserRepository
from ..db import get_db

router = APIRouter()


@router.get("/users", response_model=list[UserOut])
def list_users(db: Session = Depends(get_db)):
    return UserRepository(db).all()


@router.post("/users")
async def create_user(payload: UserOut):
    pass


def helper():
    pass


class Unrelated:
    pass
`
	byID := unitsIn(t, "api/routers/users.py", src)

	list := byID["api/routers/users#list_users"]
	require.NotNil(t, list)
	assert.Equal(t, unit.KindFunction, list.Kind)
	assert.Equal(t, []unit.Marker{decorated("get")}, list.Markers)
	for _, want := range []unit.Ref{
		{Module: "../schemas", Name: "UserOut"},
		{Module: "../repositories", Name: "UserRepository"},
		{Module: "../db", Name: "get_db"},
		{Module: "sqlalchemy.orm", Name: "Session"},
	} {
		assert.Contains(t, list.Refs, want)
	}
	assert.Equal(t, []unit.Marker{decorated("post")}, byID["api/routers/users#create_user"].Markers)
	assert.Empty(t, byID["api/routers/users#helper"].Markers)
	// The route's decorator belongs to the route. It used to land on the
	// class declared after it too, which made a Pydantic model a route.
	assert.Empty(t, byID["api/routers/users#Unrelated"].Markers)

	// The module's own line, `router = APIRouter()`, is the module's.
	mod := byID["api/routers/users#"]
	require.NotNil(t, mod)
	assert.Contains(t, mod.Refs, unit.Ref{Module: "fastapi", Name: "APIRouter"})

	assert.ElementsMatch(t, []string{"fastapi", "sqlalchemy.orm", "..schemas", "..repositories", "..db"}, rawImports(t, "api/routers/users.py", src))
}

// Pydantic's BaseModel, its BaseSettings and a dataclass are three different
// claims, and the lanes should be able to tell them apart.
func TestSchemasSettingsAndDataclasses(t *testing.T) {
	byID := unitsIn(t, "api/schemas.py", `from pydantic import BaseModel, BaseSettings
from dataclasses import dataclass


class UserOut(BaseModel):
    id: int


class Settings(BaseSettings):
    db_url: str


@dataclass
class Page:
    size: int
`)
	assert.Equal(t, []unit.Marker{super("BaseModel")}, byID["api/schemas#UserOut"].Markers)
	assert.Equal(t, []unit.Marker{super("BaseSettings")}, byID["api/schemas#Settings"].Markers)
	assert.Equal(t, []unit.Marker{decorated("dataclass")}, byID["api/schemas#Page"].Markers)
}

// A SQLAlchemy model extends the declarative Base; a repository is a plain
// class whose methods do the querying, so its references sit on the methods.
func TestSQLAlchemyModelsAndRepositories(t *testing.T) {
	models := unitsIn(t, "api/models.py", `from sqlalchemy import Column, Integer
from .db import Base


class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True)
`)
	user := models["api/models#User"]
	require.NotNil(t, user)
	// models.py is a Django name, and the marker says so; FastAPI code is
	// told apart by the base class.
	assert.ElementsMatch(t, []unit.Marker{filename("models"), super("Base")}, user.Markers)
	assert.Contains(t, user.Refs, unit.Ref{Module: "./db", Name: "Base"})
	assert.Contains(t, user.Refs, unit.Ref{Module: "sqlalchemy", Name: "Column"})

	repos := unitsIn(t, "api/repositories.py", `from sqlalchemy.orm import Session
from .models import User


class UserRepository:
    def __init__(self, db: Session):
        self.db = db

    def all(self):
        return self.db.query(User).all()
`)
	repo := repos["api/repositories#UserRepository"]
	require.NotNil(t, repo)
	assert.Empty(t, repo.Markers)
	assert.Empty(t, repo.Refs)
	assert.Equal(t, []unit.Ref{{Module: "sqlalchemy.orm", Name: "Session"}}, repos["api/repositories#UserRepository.__init__"].Refs)
	assert.Equal(t, []unit.Ref{{Module: "./models", Name: "User"}}, repos["api/repositories#UserRepository.all"].Refs)
	assert.Equal(t, "api/repositories#UserRepository", repos["api/repositories#UserRepository.all"].Owner)
}

func TestCeleryTasksAndFlaskViews(t *testing.T) {
	tasks := unitsIn(t, "api/tasks.py", `from celery import shared_task
from .repositories import UserRepository


@shared_task
def send_welcome(user_id):
    UserRepository(None)
`)
	task := tasks["api/tasks#send_welcome"]
	require.NotNil(t, task)
	assert.ElementsMatch(t, []unit.Marker{filename("tasks"), decorated("shared_task")}, task.Markers)
	assert.Contains(t, task.Refs, unit.Ref{Module: "./repositories", Name: "UserRepository"})

	flask := unitsIn(t, "web/app.py", `from flask import Blueprint
from flask.views import MethodView

bp = Blueprint("x", __name__)


@bp.route("/health")
def health():
    return "ok"


class ItemView(MethodView):
    def get(self):
        pass
`)
	assert.Equal(t, []unit.Marker{decorated("route")}, flask["web/app#health"].Markers)
	assert.Equal(t, []unit.Marker{super("MethodView")}, flask["web/app#ItemView"].Markers)
	assert.Empty(t, flask["web/app#ItemView.get"].Markers)
}

// A generic base and a keyword argument: `Repository[Product]` is a
// Repository, and `metaclass=ABCMeta` is not a base class.
func TestGenericBasesAndKeywordArguments(t *testing.T) {
	byID := unitsIn(t, "api/repos.py", `class ProductRepository(Repository[Product], metaclass=ABCMeta):
    pass


class Plain:
    pass
`)
	assert.Equal(t, []unit.Marker{super("Repository")}, byID["api/repos#ProductRepository"].Markers)
	assert.Empty(t, byID["api/repos#Plain"].Markers)
}
