package navigation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/basic"
	"github.com/archstats/archstats/extensions/treesitter/csharp"
	"github.com/archstats/archstats/extensions/treesitter/golang"
	"github.com/archstats/archstats/extensions/treesitter/java"
	"github.com/archstats/archstats/extensions/treesitter/php"
	"github.com/archstats/archstats/extensions/treesitter/python"
	"github.com/archstats/archstats/extensions/treesitter/typescript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each case is a small codebase scanned end to end, language packs included,
// since every fact here is linked to the units and functions the packs find.

func scan(t *testing.T, files map[string]string, exts ...core.Extension) *core.Results {
	t.Helper()
	root := t.TempDir()
	for p, src := range files {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o644))
	}
	results, err := core.New(&core.Config{RootPath: root, Extensions: append([]core.Extension{basic.Extension(), Extension()}, exts...)}).Analyze()
	require.NoError(t, err)
	return results
}

// rows renders a view as "col=value" lines over the named columns, sorted,
// so a case states what it expects as text.
func rows(t *testing.T, r *core.Results, view string, cols ...string) []string {
	t.Helper()
	v, err := r.RenderView(view)
	require.NoError(t, err)
	var out []string
	for _, row := range v.Rows {
		var parts []string
		for _, c := range cols {
			parts = append(parts, fmt.Sprintf("%v", row.Data[c]))
		}
		out = append(out, strings.Join(parts, " "))
	}
	sort.Strings(out)
	return out
}

func TestSpringRoutesJoinTheClassPrefix(t *testing.T) {
	r := scan(t, map[string]string{
		"src/main/java/com/acme/web/OrderController.java": `package com.acme.web;

@RestController
@RequestMapping("/" + OrderController.SECTION)
public class OrderController {
    static final String SECTION = "orders";

    @GetMapping({"", "/"})
    public List<Order> list() { return null; }

    @PostMapping(value = "/{id}/cancel")
    public void cancel(@PathVariable long id) {
        log.warn("No @RequestMapping here, only a string");
    }

    @RequestMapping(value = "/search", method = {RequestMethod.GET, RequestMethod.POST})
    public List<Order> search() { return null; }

    @KafkaListener(topics = "order-events")
    public void onEvent(String payload) {}

    @Scheduled(cron = "0 0 * * * *")
    public void sweep() {}
}
`,
		"src/main/java/com/acme/client/PaymentsClient.java": `package com.acme.client;

@FeignClient(name = "payments")
public interface PaymentsClient {
    @GetMapping("/payments/{id}")
    Payment get(@PathVariable long id);
}
`,
		"src/main/java/com/acme/client/Github.java": `package com.acme.client;

public interface Github {
    @GET("users/{user}/repos")
    Call<List<Repo>> listRepos(@Path("user") String user);
}
`,
		"src/main/java/com/acme/api/Customers.java": `package com.acme.api;

@Path("/customers")
public class Customers {
    @GET
    @Path("{id}")
    public Customer find(@PathParam("id") long id) { return null; }

    @DELETE
    public void clear() {}
}
`,
	}, &java.Extension{})
	assert.Equal(t, []string{
		"http DELETE /customers jax-rs Customers.clear com.acme.api.Customers",
		"http GET /customers/{id} jax-rs Customers.find com.acme.api.Customers",
		"http GET /{SECTION} spring OrderController.list com.acme.web.OrderController",
		"http GET,POST /{SECTION}/search spring OrderController.search com.acme.web.OrderController",
		"http POST /{SECTION}/{id}/cancel spring OrderController.cancel com.acme.web.OrderController",
		"message  order-events spring-kafka OrderController.onEvent com.acme.web.OrderController",
		"schedule  0 0 * * * * spring OrderController.sweep com.acme.web.OrderController",
	}, rows(t, r, "entry_points", "kind", "method", "path", "framework", "function", "unit"))
}

func TestCallStyleRoutes(t *testing.T) {
	r := scan(t, map[string]string{
		"server/routes/orders.js": `const express = require('express');
const { listOrders } = require('../controllers/orders');
const router = express.Router();
router.get('/orders', listOrders);
router.post('/orders/:id', requireAuth, (req, res) => res.send('ok'));
axios.get('/api/orders', { params });
module.exports = router;
`,
		"server/controllers/orders.js": `function listOrders(req, res) { res.json([]) }
module.exports = { listOrders };
`,
		"shop/urls.py": `from django.urls import path
from oscar.core.loading import get_class
from . import views

basket_view = get_class("basket.views", "BasketView")

urlpatterns = [
    path("", views.IndexView.as_view(), name="index"),
    path("basket/", basket_view.as_view(), name="basket"),
    path("api/", include("api.urls")),
]
`,
		"shop/views.py": `class IndexView:
    pass

class BasketView:
    pass
`,
		"cmd/server/main.go": `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	v1 := r.Group("/v1")
	v1.GET("/orders/:id", getOrder)
	r.POST("/login", func(c *gin.Context) {})
	c.Get("key")
}

func getOrder(c *gin.Context) {}
`,
		"cmd/tool/root.go": `package main

import "github.com/spf13/cobra"

var importCmd = &cobra.Command{
	Use:   "import [file]",
	RunE:  runImport,
}

func runImport(cmd *cobra.Command, args []string) error { return nil }
`,
	}, &typescript.Extension{}, &python.Extension{}, &golang.Extension{})
	assert.Equal(t, []string{
		"cli  import cobra runImport",
		"http ANY / django IndexView",
		"http ANY /basket django BasketView",
		"http GET /orders express listOrders",
		"http GET /v1/orders/:id gin getOrder",
		"http POST /login gin ",
		"http POST /orders/:id express ",
		"main  cmd/server go main",
	}, rows(t, r, "entry_points", "kind", "method", "path", "framework", "handler"))
	assert.Contains(t, rows(t, r, "entry_points", "kind", "handler", "unit"), "http BasketView shop/views#BasketView")
}

func TestAspNetAttributeAndConventionalRoutes(t *testing.T) {
	r := scan(t, map[string]string{
		"Web/Controllers/OrdersController.cs": `namespace Shop.Web.Controllers;

[Route("api/[controller]")]
public class OrdersController : Controller {
    [HttpGet("{id}")]
    public async Task<IActionResult> Get(int id) { return null; }

    [HttpPost]
    [Authorize]
    public IActionResult Create([FromBody] Order order) { return null; }
}
`,
		"Web/Controllers/HomeController.cs": `namespace Shop.Web.Controllers;

public class HomeController : Controller {
    public virtual IActionResult Index() { return null; }
    private IActionResult Helper() { return null; }
}
`,
	}, &csharp.Extension{})
	assert.Equal(t, []string{
		"ANY /Home/Index aspnet-conventional",
		"GET /api/Orders/{id} aspnet",
		"POST /api/Orders aspnet",
	}, rows(t, r, "entry_points", "method", "path", "framework"))
}

func TestSymfonyYamlRoutesUnderTheirImportPrefix(t *testing.T) {
	r := scan(t, map[string]string{
		"config/routes.yaml": `admin:
    resource: "@ShopAdminBundle/Resources/config/routing.yml"
    prefix: /admin
`,
		"src/Shop/Bundle/AdminBundle/Resources/config/routing.yml": `shop_admin_dashboard:
    path: /
    methods: [GET]
    defaults:
        _controller: App\Controller\DashboardController::index
`,
		"src/Controller/DashboardController.php": `<?php
namespace App\Controller;

class DashboardController {
    public function index() {}
}
`,
	}, &php.Extension{})
	assert.Equal(t, []string{"GET /admin symfony DashboardController.index"}, rows(t, r, "entry_points", "method", "path", "framework", "handler"))
}

func TestFileRoutesNeedTheirFrameworkConfig(t *testing.T) {
	files := map[string]string{
		"app/orders/[id]/page.tsx":     "export default function OrderPage() { return null }\n",
		"app/(shop)/api/cart/route.ts": "export async function GET() { return null }\n",
		"pages/index.tsx":              "export default function Home() { return null }\n",
	}
	assert.Empty(t, rows(t, scan(t, files, &typescript.Extension{}), "entry_points", "path"))
	files["next.config.js"] = "module.exports = {}\n"
	assert.Equal(t, []string{
		"http /api/cart next",
		"page / next",
		"page /orders/[id] next",
	}, rows(t, scan(t, files, &typescript.Extension{}), "entry_points", "kind", "path", "framework"))
}

func TestEntitiesAndWhoTouchesThem(t *testing.T) {
	r := scan(t, map[string]string{
		"src/main/java/com/acme/order/Order.java": `package com.acme.order;

@Entity
@Table(name = "m_order")
public class Order {}
`,
		"src/main/java/com/acme/order/OrderRepository.java": `package com.acme.order;

public interface OrderRepository extends JpaRepository<Order, Long> {
    @Query("select o from Order o where o.status = :status")
    List<Order> byStatus(String status);
}
`,
		"src/main/java/com/acme/report/Reports.java": `package com.acme.report;

public class Reports {
    private final JdbcTemplate jdbcTemplate;

    public int count() {
        String sql = "select count(*) " +
            " from m_order o join m_customer c on c.id = o.customer_id";
        return jdbcTemplate.queryForObject(sql, Integer.class);
    }

    public void archive() {
        jdbcTemplate.update("insert into m_order_archive select * from m_order");
        log.info("Failed to load data from the server, retrying");
    }
}
`,
	}, &java.Extension{})
	assert.Equal(t, []string{"Order m_order jpa com.acme.order.Order"}, rows(t, r, "data_entities", "name", "table", "framework", "entity"))
	assert.Equal(t, []string{
		"Order read jpql com.acme.order.Order com.acme.order.OrderRepository",
		"Order read_write repository com.acme.order.Order com.acme.order.OrderRepository",
		"m_customer read sql  com.acme.report.Reports",
		"m_order read sql com.acme.order.Order com.acme.report.Reports",
		"m_order read sql com.acme.order.Order com.acme.report.Reports",
		"m_order_archive write sql  com.acme.report.Reports",
	}, rows(t, r, "data_access", "target", "access", "via", "entity", "unit"))
}

func TestDjangoModelsByInheritance(t *testing.T) {
	r := scan(t, map[string]string{
		"catalogue/models.py": `from django.db import models

class AbstractProduct(models.Model):
    class Meta:
        abstract = True

class Product(AbstractProduct):
    class Meta:
        db_table = "catalogue_product"
`,
		"catalogue/views.py": `from .models import Product

def detail(request, pk):
    product = Product.objects.get(pk=pk)
    Product.objects.create(title="x")
`,
	}, &python.Extension{})
	assert.Equal(t, []string{"AbstractProduct  django", "Product catalogue_product django"}, rows(t, r, "data_entities", "name", "table", "framework"))
	assert.Equal(t, []string{"Product read orm detail", "Product write orm detail"}, rows(t, r, "data_access", "target", "access", "via", "function"))
}

func TestBindings(t *testing.T) {
	r := scan(t, map[string]string{
		"src/main/java/com/acme/pay/PaymentGateway.java": `package com.acme.pay;

public interface PaymentGateway {}
`,
		"src/main/java/com/acme/pay/StripeGateway.java": `package com.acme.pay;

@Service
public class StripeGateway implements PaymentGateway {}
`,
		"src/main/java/com/acme/pay/PaymentConfig.java": `package com.acme.pay;

@Configuration
public class PaymentConfig {
    @Bean
    public PaymentGateway fallbackGateway() {
        return new StripeGateway();
    }
}
`,
		"src/main/resources/applicationContext.xml": `<beans xmlns="http://www.springframework.org/schema/beans">
    <bean id="gateway" class="com.acme.pay.StripeGateway"/>
</beans>
`,
	}, &java.Extension{})
	assert.Equal(t, []string{
		"PaymentGateway com.acme.pay.PaymentGateway StripeGateway com.acme.pay.StripeGateway bean",
		"PaymentGateway com.acme.pay.PaymentGateway StripeGateway com.acme.pay.StripeGateway component_scan",
		"gateway  com.acme.pay.StripeGateway com.acme.pay.StripeGateway spring_xml",
	}, rows(t, r, "bindings", "interface", "interface_unit", "implementation", "implementation_unit", "mechanism"))
	assert.Contains(t, rows(t, r, "unit_supertypes", "unit", "supertype", "name"), "com.acme.pay.StripeGateway com.acme.pay.PaymentGateway PaymentGateway")
}

func TestDocsAndWhatTheyAreAbout(t *testing.T) {
	r := scan(t, map[string]string{
		"docs/adr/0003-split-payments.md":                "# 3. Split payments out of orders\n\nDate: 2025-04-01\n\n## Status\n\nAccepted\n\n## Context\n\n`PaymentGateway` lives in src/main/java/com/acme/pay.\n\n## Decision\n\nKeep it.\n",
		"src/main/java/com/acme/pay/README.md":           "# Payments\n\nHow payments work.\n",
		"src/main/java/com/acme/pay/PaymentGateway.java": "package com.acme.pay;\n\npublic interface PaymentGateway {}\n",
	}, &java.Extension{})
	assert.Equal(t, []string{
		"docs/adr/0003-split-payments.md adr 3. Split payments out of orders Accepted 2025-04-01",
		"src/main/java/com/acme/pay/README.md readme Payments  ",
	}, rows(t, r, "docs", "file", "kind", "title", "status", "date"))
	assert.Equal(t, []string{
		"docs/adr/0003-split-payments.md com.acme.pay names_path",
		"docs/adr/0003-split-payments.md com.acme.pay names_unit",
		"src/main/java/com/acme/pay/README.md com.acme.pay located_in",
	}, rows(t, r, "doc_links", "doc", "component", "how"))
}

func TestUnitsCarryLineSignatureAndRank(t *testing.T) {
	r := scan(t, map[string]string{
		"src/main/java/com/acme/core/Money.java":     "package com.acme.core;\n\npublic final class Money {\n    public Money plus(Money other) { return this; }\n}\n",
		"src/main/java/com/acme/a/A.java":            "package com.acme.a;\n\nimport com.acme.core.Money;\n\npublic class A { Money m; }\n",
		"src/main/java/com/acme/b/B.java":            "package com.acme.b;\n\nimport com.acme.core.Money;\n\npublic class B { Money m; }\n",
		"src/test/java/com/acme/core/MoneyTest.java": "package com.acme.core;\n\npublic class MoneyTest { Money m; }\n",
	}, &java.Extension{})
	got := rows(t, r, "units", "name", "line", "signature", "used_by", "used_by_components")
	assert.Contains(t, got, "Money 3 public final class Money 2 2")
	v, _ := r.RenderView("units")
	ranks := map[string]float64{}
	for _, row := range v.Rows {
		ranks[row.Data["name"].(string)] = row.Data["page_rank"].(float64)
	}
	assert.Greater(t, ranks["Money"], ranks["A"])
	assert.Zero(t, ranks["MoneyTest"], "a test is not ranked")
}
