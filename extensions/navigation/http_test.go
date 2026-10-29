package navigation

import (
	"testing"

	"github.com/archstats/archstats/extensions/treesitter/csharp"
	"github.com/archstats/archstats/extensions/treesitter/golang"
	"github.com/archstats/archstats/extensions/treesitter/java"
	"github.com/archstats/archstats/extensions/treesitter/php"
	"github.com/archstats/archstats/extensions/treesitter/python"
	"github.com/archstats/archstats/extensions/treesitter/typescript"
	"github.com/stretchr/testify/assert"
)

// Each case is one small service in one framework: its routes, what they
// reach, what they call out to, and where that is configured.

func TestSpringRouteTraceContractAndOutbound(t *testing.T) {
	r := scan(t, map[string]string{
		"src/main/java/com/acme/orders/OrderController.java": `package com.acme.orders;

@RestController
@RequestMapping("/api/v1/orders")
public class OrderController {
    private final OrderService orders;

    public OrderController(OrderService orders) { this.orders = orders; }

    @GetMapping("/{id}")
    public Order get(@PathVariable long id) {
        return orders.find(id);
    }

    @PreAuthorize("hasRole('ADMIN')")
    @PostMapping("/{id}/cancel")
    public void cancel(@PathVariable long id) {
        orders.cancel(id);
    }

    @GetMapping("/openOrders")
    public List<Order> open() { return List.of(); }
}
`,
		"src/main/java/com/acme/orders/OrderService.java": `package com.acme.orders;

public interface OrderService {
    Order find(long id);
    void cancel(long id);
}
`,
		"src/main/java/com/acme/orders/DefaultOrderService.java": `package com.acme.orders;

@Service
public class DefaultOrderService implements OrderService {
    private final OrderRepository repository;
    private final PaymentsClient payments;
    private final RestTemplate restTemplate;
    @Value("${ledger.base-url}")
    private String ledgerUrl;

    public Order find(long id) {
        return repository.findById(id).orElseThrow();
    }

    public void cancel(long id) {
        Order order = repository.findById(id).orElseThrow();
        payments.refund(order.getPaymentId());
        restTemplate.postForObject(ledgerUrl + "/entries", order, Void.class);
    }
}
`,
		"src/main/java/com/acme/orders/OrderRepository.java": `package com.acme.orders;

public interface OrderRepository extends JpaRepository<Order, Long> {
}
`,
		"src/main/java/com/acme/orders/Order.java": `package com.acme.orders;

@Entity
@Table(name = "orders")
public class Order {
    @Id private Long id;
    public String getPaymentId() { return null; }
}
`,
		"src/main/java/com/acme/orders/PaymentsClient.java": `package com.acme.orders;

@FeignClient(name = "payments", url = "${clients.payments.url}")
public interface PaymentsClient {
    @PostMapping("/refunds/{paymentId}")
    void refund(@PathVariable String paymentId);
}
`,
		"src/main/java/com/acme/config/SecurityConfig.java": `package com.acme.config;

@Configuration
public class SecurityConfig {
    @Bean
    public SecurityFilterChain api(HttpSecurity http) throws Exception {
        http.securityMatcher("/api/**").authorizeHttpRequests(auth -> auth
            .requestMatchers(HttpMethod.GET, "/api/v1/orders/**").permitAll()
            .anyRequest().authenticated());
        return http.build();
    }

    @Bean
    public SecurityFilterChain docs(HttpSecurity http) throws Exception {
        http.securityMatcher("/swagger-ui/**").authorizeHttpRequests(auth -> auth.anyRequest().permitAll());
        return http.build();
    }
}
`,
		"src/main/resources/application.yml": `spring:
  datasource:
    url: jdbc:postgresql://localhost:5432/orders
    username: orders
    password: s3cret
clients:
  payments:
    url: http://localhost:8081
ledger:
  base-url: ${LEDGER_URL:http://localhost:8082}
feign:
  client:
    config:
      payments:
        readTimeout: 2000
`,
		"src/main/resources/application-prod.yml": `spring:
  datasource:
    url: jdbc:postgresql://orders-db.internal:5432/orders
clients:
  payments:
    url: https://payments.acme.nl/api
`,
		"api/orders.yaml": `openapi: 3.0.3
info:
  title: Orders
  version: "1.0"
servers:
  - url: http://api.acme.nl/api/v1
paths:
  /orders/{id}:
    get:
      operationId: getOrder
      responses:
        "200":
          description: ok
        "404":
          description: missing
          content:
            application/json: {}
  /orders/{id}/refund:
    post:
      operationId: refundOrder
      responses:
        "200":
          description: ok
`,
	}, &java.Extension{})

	assert.Equal(t, []string{
		"GET /api/v1/orders/openOrders /api/v1/orders/openOrders permitAll ",
		"GET /api/v1/orders/{id} /api/v1/orders/{} permitAll spec",
		`POST /api/v1/orders/{id}/cancel /api/v1/orders/{}/cancel PreAuthorize("hasRole('ADMIN')") `,
	}, rows(t, r, "entry_points", "method", "path", "template", "auth", "documented"))

	// The trace follows the interface to its implementation, and stops at
	// the repository and the client, which have no bodies.
	calls := rows(t, r, "calls", "from_function", "to_function", "via")
	assert.Contains(t, calls, "OrderController.get OrderService.find field")
	assert.Contains(t, calls, "OrderController.get DefaultOrderService.find implementation")
	assert.Contains(t, calls, "DefaultOrderService.find OrderRepository.findById field")
	assert.Contains(t, calls, "DefaultOrderService.cancel PaymentsClient.refund field")

	assert.Equal(t, []string{
		"http feign POST /api/refunds/{paymentId} payments payments.acme.nl declared",
		"http resttemplate POST /entries ledger localhost resolved",
	}, rows(t, r, "outbound_calls", "kind", "client", "method", "path", "system", "host", "confidence"))
	assert.Contains(t, rows(t, r, "outbound_calls", "client", "resilience"), "feign timeout")

	systems := rows(t, r, "outbound_systems", "system", "kind", "environment", "host")
	assert.Contains(t, systems, "payments http prod payments.acme.nl")
	assert.Contains(t, systems, "payments http  localhost")
	assert.Contains(t, systems, "sql:orders sql prod orders-db.internal")

	// No secret is kept.
	for _, row := range rows(t, r, "config_entries", "key", "value") {
		assert.NotContains(t, row, "s3cret")
		assert.NotContains(t, row, "username")
	}

	assert.Equal(t, []string{
		"GET /orders/{id} getOrder 2",
		"POST /orders/{id}/refund refundOrder 0",
	}, rows(t, r, "api_operations", "method", "path", "operation_id", "entry"))
	findings := rows(t, r, "api_findings", "rule", "subject", "path")
	assert.Contains(t, findings, "servers-use-https spec ")
	assert.Contains(t, findings, "missing-version-header spec ")
	assert.Contains(t, findings, "use-problem-schema spec ")
	assert.Contains(t, findings, "paths-kebab-case route /api/v1/orders/openOrders")
	assert.NotContains(t, findings, "include-major-version-in-uri spec ")

	// The cancel route reaches the orders table and two outbound calls.
	assert.Contains(t, rows(t, r, "route_reach", "entry", "tables", "outbound", "unknown"), "3 1 2 0")
}

func TestAspNetTypedClientsAndRefit(t *testing.T) {
	r := scan(t, map[string]string{
		"Web/Controllers/OrdersController.cs": `namespace Shop.Web.Controllers;

[ApiController]
[Authorize(Roles = "Clerk")]
[Route("api/v1/orders")]
public class OrdersController : ControllerBase {
    private readonly IOrderService _orders;
    public OrdersController(IOrderService orders) { _orders = orders; }

    [HttpGet("{id}")]
    public async Task<IActionResult> Get(int id) {
        var order = await _orders.FindAsync(id);
        return Ok(order);
    }

    [AllowAnonymous]
    [HttpGet("health")]
    public IActionResult Health() => Ok();
}
`,
		"Web/Services/OrderService.cs": `namespace Shop.Web.Services;

public class OrderService : IOrderService {
    private readonly HttpClient _http;
    public OrderService(HttpClient http) { _http = http; }

    public async Task<Order> FindAsync(int id) {
        return await _http.GetFromJsonAsync<Order>($"orders/{id}");
    }
}
`,
		"Web/Services/IOrderService.cs": `namespace Shop.Web.Services;

public interface IOrderService {
    Task<Order> FindAsync(int id);
}
`,
		"Web/Clients/IStockApi.cs": `namespace Shop.Web.Clients;

public interface IStockApi {
    [Get("/stock/{sku}")]
    Task<Stock> GetStock(string sku);
}
`,
		"Web/Program.cs": `var builder = WebApplication.CreateBuilder(args);
builder.Services.AddHttpClient<IOrderService, OrderService>(c => c.BaseAddress = new Uri(builder.Configuration["Backend:BaseUrl"]))
    .AddStandardResilienceHandler();
builder.Services.AddRefitClient<IStockApi>().ConfigureHttpClient(c => c.BaseAddress = new Uri(builder.Configuration["Stock:Url"]));
var app = builder.Build();
app.MapGet("/api/v1/ping", () => "pong").RequireAuthorization("Ops");
app.Run();
`,
		"Web/appsettings.json": `{
  // comments are allowed here
  "Backend": { "BaseUrl": "http://localhost:5001/" },
  "Stock": { "Url": "http://localhost:5002" },
  "ConnectionStrings": { "Default": "Server=localhost;Database=Shop;User Id=sa;Password=Secret1;" }
}`,
		"Web/appsettings.Production.json": `{
  "Backend": { "BaseUrl": "https://backend.shop.nl/" },
  "Stock": { "Url": "https://stock.shop.nl" }
}`,
	}, &csharp.Extension{})

	assert.Equal(t, []string{
		" Web dotnet ",
		"GET /api/v1/orders/health aspnet AllowAnonymous",
		`GET /api/v1/orders/{id} aspnet Authorize(Roles = "Clerk")`,
		`GET /api/v1/ping aspnet-minimal RequireAuthorization("Ops")`,
	}, rows(t, r, "entry_points", "method", "path", "framework", "auth"))

	assert.Equal(t, []string{
		"httpclient GET /orders/{id} backend backend.shop.nl retry,circuit_breaker",
		"refit GET /stock/{sku} stock stock.shop.nl ",
	}, rows(t, r, "outbound_calls", "client", "method", "path", "system", "host", "resilience"))
	assert.Contains(t, rows(t, r, "calls", "from_function", "to_function", "via"), "OrdersController.Get OrderService.FindAsync implementation")
	for _, row := range rows(t, r, "config_entries", "value") {
		assert.NotContains(t, row, "Secret1")
		assert.NotContains(t, row, "sa")
	}
	assert.Contains(t, rows(t, r, "outbound_systems", "system", "kind", "host"), "sql:Shop sql localhost")
}

func TestExpressNestAndFrontendSelfCalls(t *testing.T) {
	r := scan(t, map[string]string{
		"server/routes/orders.ts": `import express from 'express';
import { listOrders, refund } from '../services/orders';
const router = express.Router();
router.get('/api/orders', requireAuth, async (req, res) => {
  res.json(await listOrders());
});
router.post('/api/orders/:id/refund', async (req, res) => {
  await refund(req.params.id);
  res.sendStatus(204);
});
export default router;
`,
		"server/services/orders.ts": `import axios from 'axios';
const payments = axios.create({ baseURL: process.env.PAYMENTS_URL, timeout: 3000 });

export async function listOrders() {
  return db.query('select * from orders');
}

export async function refund(id: string) {
  await payments.post(` + "`" + `/refunds/${id}` + "`" + `);
}
`,
		"server/.env.production": "PAYMENTS_URL=https://payments.example.com\n",
		"web/src/api.ts": `export async function loadOrders() {
  const res = await fetch('/api/orders');
  return res.json();
}
`,
		"api/src/stock.controller.ts": `import { Controller, Get, Param, UseGuards } from '@nestjs/common';
import { HttpService } from '@nestjs/axios';

@Controller('stock')
@UseGuards(JwtGuard)
export class StockController {
  constructor(private readonly http: HttpService, private readonly stock: StockService) {}

  @Get(':sku')
  find(@Param('sku') sku: string) {
    return this.stock.lookup(sku);
  }
}
`,
		"api/src/stock.service.ts": `import { Injectable } from '@nestjs/common';
import { HttpService } from '@nestjs/axios';

@Injectable()
export class StockService {
  constructor(private readonly http: HttpService) {}

  lookup(sku: string) {
    return this.http.get(` + "`" + `${process.env.WAREHOUSE_URL}/items/${sku}` + "`" + `);
  }
}
`,
	}, &typescript.Extension{})

	assert.Equal(t, []string{
		"GET /api/orders requireAuth",
		"GET /stock/:sku UseGuards(JwtGuard)",
		"POST /api/orders/:id/refund ",
	}, rows(t, r, "entry_points", "method", "path", "auth"))

	assert.Equal(t, []string{
		"axios POST /refunds/{id} payments payments.example.com resolved timeout 0",
		"fetch GET /api/orders self  resolved  1",
		"nestjs-http GET /items/{sku} warehouse  partial  0",
	}, rows(t, r, "outbound_calls", "client", "method", "path", "system", "host", "confidence", "resilience", "entry"))
	calls := rows(t, r, "calls", "from_function", "to_function", "via")
	assert.Contains(t, calls, "StockController.find StockService.lookup field")
	assert.Contains(t, rows(t, r, "route_reach", "entry", "outbound"), "3 1")
}

func TestPythonFastAPIDjangoAndCelery(t *testing.T) {
	r := scan(t, map[string]string{
		"app/api.py": `from fastapi import APIRouter, Depends
from app.services import OrderService
import requests

router = APIRouter()

@router.get("/api/v1/orders/{order_id}")
def get_order(order_id: int, service: OrderService = Depends()):
    return service.find(order_id)

@router.post("/api/v1/orders/{order_id}/ship")
def ship(order_id: int):
    requests.post(f"{settings.SHIPPING_URL}/shipments", json={"id": order_id}, timeout=5)
    send_confirmation.delay(order_id)
`,
		"app/services.py": `import httpx

class OrderService:
    def __init__(self):
        self.client = httpx.Client(base_url=os.environ["CATALOG_URL"])

    def find(self, order_id):
        return self.client.get(f"/products/{order_id}")
`,
		"app/tasks.py": `from celery import shared_task

@shared_task
def send_confirmation(order_id):
    pass
`,
		"shop/views.py": `from rest_framework.views import APIView
from rest_framework.permissions import IsAuthenticated

class BasketView(APIView):
    permission_classes = [IsAuthenticated]

    def get(self, request):
        return None
`,
		"shop/urls.py": `from django.urls import path
from django.contrib.auth.decorators import login_required
from . import views

urlpatterns = [
    path("basket/", views.BasketView.as_view()),
    path("account/", login_required(views.AccountView.as_view())),
]
`,
		"config/settings.py": `SHIPPING_URL = os.environ.get("SHIPPING_URL", "http://localhost:9000")
DATABASES = {
    "default": {
        "ENGINE": "django.db.backends.postgresql",
        "HOST": "db.internal",
        "PASSWORD": "hunter2",
    }
}
`,
	}, &python.Extension{})

	assert.Equal(t, []string{
		"  ",
		"ANY /account login_required",
		"ANY /basket permission_classes(IsAuthenticated)",
		"GET /api/v1/orders/{order_id} ",
		"POST /api/v1/orders/{order_id}/ship ",
	}, rows(t, r, "entry_points", "method", "path", "auth"))
	assert.Contains(t, rows(t, r, "entry_points", "path", "documented"), "/api/v1/orders/{order_id} framework")

	assert.Equal(t, []string{
		"http httpx GET /products/{order_id} catalog ",
		"http requests POST /shipments shipping timeout",
		"message celery TASK send_confirmation celery ",
	}, rows(t, r, "outbound_calls", "kind", "client", "method", "path", "system", "resilience"))
	assert.Contains(t, rows(t, r, "outbound_calls", "client", "consumer"), "celery 5")
	assert.Contains(t, rows(t, r, "calls", "from_function", "to_function", "via"), "get_order OrderService.find field")
	for _, row := range rows(t, r, "config_entries", "value") {
		assert.NotContains(t, row, "hunter2")
	}
}

func TestGoHandlersAndClients(t *testing.T) {
	r := scan(t, map[string]string{
		"go.mod": "module example.com/shop\n\ngo 1.22\n",
		"cmd/api/main.go": `package main

import (
	"net/http"

	"example.com/shop/orders"
)

func main() {
	h := orders.NewHandler(orders.NewService())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/orders/{id}", h.Get)
	http.ListenAndServe(":8080", mux)
}
`,
		"orders/handler.go": `package orders

import "net/http"

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	h.svc.Find(r.PathValue("id"))
}
`,
		"orders/service.go": `package orders

import (
	"net/http"
	"os"
)

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) Find(id string) error {
	req, _ := http.NewRequest(http.MethodGet, os.Getenv("INVENTORY_URL")+"/items/"+id, nil)
	_, err := http.DefaultClient.Do(req)
	return err
}
`,
		"deploy/values-prod.yaml": "env:\n  - name: INVENTORY_URL\n    value: https://inventory.shop.nl\n",
	}, &golang.Extension{})

	assert.Equal(t, []string{" cmd/api go main", "GET /api/v1/orders/{id} net/http Get"}, rows(t, r, "entry_points", "method", "path", "framework", "handler"))
	assert.Contains(t, rows(t, r, "calls", "from_function", "to_function", "via"), "Handler.Get Service.Find field")
	assert.Equal(t, []string{"net/http GET /items/{id} inventory inventory.shop.nl"}, rows(t, r, "outbound_calls", "client", "method", "path", "system", "host"))
}

func TestLaravelGroupsAndHTTPFacade(t *testing.T) {
	r := scan(t, map[string]string{
		"routes/api.php": `<?php
use App\Http\Controllers\OrderController;

Route::middleware('auth:sanctum')->group(function () {
    Route::get('/v1/orders/{order}', [OrderController::class, 'show']);
});
Route::get('/v1/status', [OrderController::class, 'status']);
`,
		"app/Http/Controllers/OrderController.php": `<?php
namespace App\Http\Controllers;

use App\Services\Shipping;

class OrderController extends Controller
{
    public function __construct(private Shipping $shipping) {}

    public function show($order)
    {
        return $this->shipping->track($order);
    }

    public function status() { return 'ok'; }
}
`,
		"app/Services/Shipping.php": `<?php
namespace App\Services;

use Illuminate\Support\Facades\Http;

class Shipping
{
    public function track($order)
    {
        return Http::baseUrl(config('services.shipping.url'))->timeout(5)->get('/tracking/' . $order);
    }
}
`,
		"config/services.php": `<?php
return [
    'shipping' => [
        'url' => env('SHIPPING_URL', 'http://localhost:7000'),
        'key' => env('SHIPPING_KEY'),
    ],
];
`,
		".env.example": "SHIPPING_URL=https://shipping.example.nl\nSHIPPING_KEY=abc\n",
	}, &php.Extension{})

	assert.Equal(t, []string{
		"GET /v1/orders/{order} middleware('auth:sanctum')",
		"GET /v1/status ",
	}, rows(t, r, "entry_points", "method", "path", "auth"))
	assert.Contains(t, rows(t, r, "calls", "from_function", "to_function", "via"), "OrderController.show Shipping.track field")
	assert.Equal(t, []string{"laravel-http GET /tracking/{order} shipping shipping.example.nl timeout"}, rows(t, r, "outbound_calls", "client", "method", "path", "system", "host", "resilience"))
}

func TestTemplatesAndMatching(t *testing.T) {
	for in, want := range map[string]string{
		"/orders/{id}":                   "/orders/{}",
		"/orders/:id/":                   "/orders/{}",
		"orders/<int:pk>/edit":           "/orders/{}/edit",
		"^orders/(?P<pk>[0-9]+)/$":       "/orders/{}",
		"/blog/[...slug]":                "/blog/{}",
		"https://api.acme.nl/v1/x?y=1":   "/v1/x",
		"/files/{name}.json":             "/files/{}.json",
		"/api/orders/${id}/lines":        "/api/orders/{}/lines",
		"{var:id}/x":                     "/{}/x",
		"/items/{id:int}":                "/items/{}",
	} {
		assert.Equal(t, want, pathTemplate(in), in)
	}
	assert.True(t, antMatch("/api/**", "/api/v1/orders/{}"))
	assert.True(t, antMatch("/api/*/orders", "/api/v1/orders"))
	assert.False(t, antMatch("/api/*/orders", "/api/v1/v2/orders"))
	assert.True(t, antMatch("/**", "/"))
	assert.Equal(t, "prod", environmentOf("src/main/resources/application-prod.yml"))
	assert.Equal(t, "acc", environmentOf("appsettings.Acceptance.json"))
	assert.Equal(t, "", environmentOf(".env.example"))
	assert.Equal(t, "test", environmentOf("deploy/overlays/test/config.env"))
	assert.Equal(t, "orders-db.internal", hostOf("jdbc:postgresql://orders-db.internal:5432/orders"))
	assert.Equal(t, "sqlsrv", hostOf("Server=tcp:sqlsrv,1433;Database=Shop"))
	assert.Equal(t, "broker1", hostOf("broker1:9092,broker2:9092"))
	assert.Equal(t, "payments", nameFromKey("clients.payments.base-url"))
	assert.Equal(t, "partner-kvk", nameFromKey("PARTNER_KVK_URL"))
	assert.Equal(t, "http://h/x", sanitize("http://user:pw@h/x"))
}
