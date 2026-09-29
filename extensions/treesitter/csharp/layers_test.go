package csharp

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An ASP.NET Core service, read the way the lanes read it. The UI's layers
// test (archstats-ui, layers.server.test.ts) is built from what these
// assert: the markers with their source, the raw imports, and the refs that
// become edges between layers.

func super(key string) unit.Marker     { return unit.Marker{Source: unit.SourceSupertype, Key: key} }
func attribute(key string) unit.Marker { return unit.Marker{Source: unit.SourceAnnotation, Key: key} }
func keyword(key string) unit.Marker   { return unit.Marker{Source: sourceKeyword, Key: key} }

func analysed(t *testing.T, path, src string) (map[string]*unit.Unit, []string) {
	t.Helper()
	res := (&csharpAnalyzer{lp: createCSharpLanguagePack()}).lp.AnalyzeFileContent(path, []byte(src))
	require.NotNilf(t, res, "%s was not analysed", path)
	byID := map[string]*unit.Unit{}
	for _, u := range unitsFrom(path, res) {
		byID[u.ID] = u
	}
	var raw []string
	for _, s := range res.Snippets {
		if s.Type == file.ImportRaw {
			raw = append(raw, s.Value)
		}
	}
	return byID, raw
}

func refNames(u *unit.Unit) map[string]bool {
	out := map[string]bool{}
	for _, r := range u.Refs {
		out[r.Name] = true
	}
	return out
}

// The attributes on a controller's actions are the controller's. They used
// to belong to the next type declared after them, which in a file with one
// class is nothing: `[HttpPost]` never marked a controller.
func TestControllersCarryTheirActionAttributes(t *testing.T) {
	byID, raw := analysed(t, "src/Shop.Api/Controllers/OrdersController.cs", `using Microsoft.AspNetCore.Mvc;
using Shop.Api.Dtos;
using Shop.Api.Services;

namespace Shop.Api.Controllers;

[ApiController]
[Route("api/[controller]")]
public class OrdersController : ControllerBase
{
    private readonly IOrderService _orders;

    public OrdersController(IOrderService orders)
    {
        _orders = orders;
    }

    [HttpGet("{id}")]
    public ActionResult<OrderDto> Get(int id) => Ok(_orders.Get(id));

    [HttpPost]
    public IActionResult Create([FromBody] CreateOrderRequest request)
    {
        return Ok();
    }
}
`)
	c := byID["Shop.Api.Controllers.OrdersController"]
	require.NotNil(t, c)
	assert.Equal(t, unit.KindType, c.Kind)
	assert.Empty(t, c.Owner)
	assert.ElementsMatch(t, []unit.Marker{
		attribute("ApiController"), attribute("Route"), attribute("HttpGet"), attribute("HttpPost"), attribute("FromBody"),
		super("ControllerBase"),
	}, c.Markers)
	// A use is offered in every namespace the file can see; resolution
	// keeps the one that exists.
	assert.Contains(t, c.Refs, unit.Ref{Module: "Shop.Api.Services", Name: "IOrderService"})
	assert.Contains(t, c.Refs, unit.Ref{Module: "Shop.Api.Dtos", Name: "OrderDto"})
	assert.Contains(t, c.Refs, unit.Ref{Module: "Shop.Api.Dtos", Name: "CreateOrderRequest"})
	assert.ElementsMatch(t, []string{"Microsoft.AspNetCore.Mvc", "Shop.Api.Dtos", "Shop.Api.Services"}, raw)
}

// `[ApiControllerAttribute]` is `[ApiController]` to the compiler, and a
// base written with its namespace is known by its last segment only.
func TestAttributeSuffixAndQualifiedBases(t *testing.T) {
	byID, _ := analysed(t, "src/Shop.Api/Controllers/LegacyController.cs", `namespace Shop.Api.Controllers
{
    [ApiControllerAttribute]
    public class LegacyController : Microsoft.AspNetCore.Mvc.Controller
    {
    }

    public class TypedRepository : Shop.Api.Data.Repository<LegacyController>
    {
    }
}
`)
	assert.ElementsMatch(t, []unit.Marker{attribute("ApiController"), super("Controller")}, byID["Shop.Api.Controllers.LegacyController"].Markers)
	assert.Equal(t, []unit.Marker{super("Repository")}, byID["Shop.Api.Controllers.TypedRepository"].Markers)
}

func TestServicesAndInterfaces(t *testing.T) {
	byID, _ := analysed(t, "src/Shop.Api/Services/OrderService.cs", `using Shop.Api.Data;
using Shop.Api.Domain;
using Shop.Api.Dtos;

namespace Shop.Api.Services;

public interface IOrderService
{
    OrderDto Get(int id);
}

public class OrderService : IOrderService
{
    private readonly ShopDbContext _db;

    public OrderService(ShopDbContext db)
    {
        _db = db;
    }

    public OrderDto Get(int id)
    {
        Order order = _db.Orders.Find(id);
        return new OrderDto(order.Id);
    }
}
`)
	i := byID["Shop.Api.Services.IOrderService"]
	require.NotNil(t, i)
	assert.Equal(t, []unit.Marker{super("interface")}, i.Markers)
	s := byID["Shop.Api.Services.OrderService"]
	require.NotNil(t, s)
	assert.Equal(t, []unit.Marker{super("IOrderService")}, s.Markers)
	assert.Contains(t, s.Refs, unit.Ref{Module: "Shop.Api.Data", Name: "ShopDbContext"})
	assert.Contains(t, s.Refs, unit.Ref{Module: "Shop.Api.Domain", Name: "Order"})
	assert.Contains(t, s.Refs, unit.Ref{Module: "Shop.Api.Dtos", Name: "OrderDto"})
	assert.NotContains(t, refNames(s), "OrderService", "a type naming itself is not a dependency")
}

func TestDbContextAndEntityConfiguration(t *testing.T) {
	byID, raw := analysed(t, "src/Shop.Api/Data/ShopDbContext.cs", `using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using Shop.Api.Domain;

namespace Shop.Api.Data;

public class ShopDbContext : DbContext
{
    public DbSet<Order> Orders => Set<Order>();

    protected override void OnModelCreating(ModelBuilder builder)
    {
        builder.ApplyConfiguration(new OrderConfiguration());
    }
}

public class OrderConfiguration : IEntityTypeConfiguration<Order>
{
    public void Configure(EntityTypeBuilder<Order> builder)
    {
    }
}
`)
	db := byID["Shop.Api.Data.ShopDbContext"]
	require.NotNil(t, db)
	assert.Equal(t, []unit.Marker{super("DbContext")}, db.Markers)
	assert.Contains(t, db.Refs, unit.Ref{Module: "Shop.Api.Domain", Name: "Order"})
	assert.Contains(t, db.Refs, unit.Ref{Module: "Shop.Api.Data", Name: "OrderConfiguration"})
	assert.Equal(t, []unit.Marker{super("IEntityTypeConfiguration")}, byID["Shop.Api.Data.OrderConfiguration"].Markers)
	assert.Contains(t, raw, "Microsoft.EntityFrameworkCore")
}

// `[Key]` on a property is a fact about the entity, not about the abstract
// base declared after it.
func TestEntitiesCarryTheirPropertyAttributes(t *testing.T) {
	byID, _ := analysed(t, "src/Shop.Api/Domain/Order.cs", `using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace Shop.Api.Domain;

[Table("orders")]
public class Order : BaseEntity
{
    [Key]
    public int Id { get; set; }

    public decimal Total { get; set; }
}

public abstract class BaseEntity
{
}
`)
	o := byID["Shop.Api.Domain.Order"]
	require.NotNil(t, o)
	assert.ElementsMatch(t, []unit.Marker{attribute("Table"), attribute("Key"), super("BaseEntity")}, o.Markers)
	assert.Empty(t, byID["Shop.Api.Domain.BaseEntity"].Markers, "[Key] belongs to Order")
}

// The planted violation: an entity that reaches up into a service.
func TestAnEntityCallingAServiceIsARefLikeAnyOther(t *testing.T) {
	byID, _ := analysed(t, "src/Shop.Api/Domain/Customer.cs", `using Shop.Api.Services;

namespace Shop.Api.Domain;

public class Customer : BaseEntity
{
    public void Notify(IOrderService orders)
    {
    }
}
`)
	assert.Contains(t, byID["Shop.Api.Domain.Customer"].Refs, unit.Ref{Module: "Shop.Api.Services", Name: "IOrderService"})
}

// A record, a struct and an enum say what they are by their keyword, which
// is the only thing about a DTO that does.
func TestRecordsStructsAndEnumsAreMarkedByKeyword(t *testing.T) {
	byID, _ := analysed(t, "src/Shop.Api/Dtos/OrderDto.cs", `namespace Shop.Api.Dtos;

public record OrderDto(int Id);

public record CreateOrderRequest(decimal Total);

public readonly struct Money
{
    public decimal Amount { get; init; }
}

[Flags]
public enum OrderState { New = 1, Paid = 2 }

public class Plain
{
}
`)
	assert.Equal(t, []unit.Marker{keyword("record")}, byID["Shop.Api.Dtos.OrderDto"].Markers)
	assert.Equal(t, []unit.Marker{keyword("record")}, byID["Shop.Api.Dtos.CreateOrderRequest"].Markers)
	assert.Equal(t, []unit.Marker{keyword("struct")}, byID["Shop.Api.Dtos.Money"].Markers)
	assert.ElementsMatch(t, []unit.Marker{attribute("Flags"), keyword("enum")}, byID["Shop.Api.Dtos.OrderState"].Markers)
	assert.Empty(t, byID["Shop.Api.Dtos.Plain"].Markers)
}

// A MediatR command is a record implementing IRequest; its handler names it.
func TestMediatRRequestsAndHandlers(t *testing.T) {
	byID, _ := analysed(t, "src/Shop.Api/Features/Orders/CreateOrder.cs", `using MediatR;
using Shop.Api.Data;

namespace Shop.Api.Features.Orders;

public record CreateOrder(decimal Total) : IRequest<int>;

public class CreateOrderHandler : IRequestHandler<CreateOrder, int>
{
    private readonly ShopDbContext _db;

    public CreateOrderHandler(ShopDbContext db)
    {
        _db = db;
    }

    public Task<int> Handle(CreateOrder request, CancellationToken ct) => Task.FromResult(1);
}
`)
	assert.ElementsMatch(t, []unit.Marker{keyword("record"), super("IRequest")}, byID["Shop.Api.Features.Orders.CreateOrder"].Markers)
	h := byID["Shop.Api.Features.Orders.CreateOrderHandler"]
	require.NotNil(t, h)
	assert.Equal(t, []unit.Marker{super("IRequestHandler")}, h.Markers)
	assert.Contains(t, h.Refs, unit.Ref{Module: "Shop.Api.Features.Orders", Name: "CreateOrder"})
	assert.Contains(t, h.Refs, unit.Ref{Module: "Shop.Api.Data", Name: "ShopDbContext"})
}

func TestMiddlewareFiltersAndRazorPages(t *testing.T) {
	byID, _ := analysed(t, "src/Shop.Api/Pipeline/ErrorHandlingMiddleware.cs", `using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc.Filters;

namespace Shop.Api.Pipeline;

public class ErrorHandlingMiddleware : IMiddleware
{
    public Task InvokeAsync(HttpContext context, RequestDelegate next) => next(context);
}

public class AuditFilter : IActionFilter
{
    public void OnActionExecuting(ActionExecutingContext context) { }
    public void OnActionExecuted(ActionExecutedContext context) { }
}

public class TenantAttribute : Attribute
{
}
`)
	assert.Equal(t, []unit.Marker{super("IMiddleware")}, byID["Shop.Api.Pipeline.ErrorHandlingMiddleware"].Markers)
	assert.Equal(t, []unit.Marker{super("IActionFilter")}, byID["Shop.Api.Pipeline.AuditFilter"].Markers)
	// The base class Attribute is a base class, not an attribute usage, and
	// keeps its name.
	assert.Equal(t, []unit.Marker{super("Attribute")}, byID["Shop.Api.Pipeline.TenantAttribute"].Markers)

	pages, _ := analysed(t, "src/Shop.Api/Pages/Index.cshtml.cs", `using Microsoft.AspNetCore.Mvc.RazorPages;
using Shop.Api.Services;

namespace Shop.Api.Pages;

public class IndexModel : PageModel
{
    private readonly IOrderService _orders;

    public IndexModel(IOrderService orders)
    {
        _orders = orders;
    }

    public void OnGet() { }
}
`)
	p := pages["Shop.Api.Pages.IndexModel"]
	require.NotNil(t, p)
	assert.Equal(t, []unit.Marker{super("PageModel")}, p.Markers)
	assert.Contains(t, p.Refs, unit.Ref{Module: "Shop.Api.Services", Name: "IOrderService"})
}

// A minimal API's Program.cs declares no type. Its top-level statements are
// where the routes are mapped and the services registered, and they belong
// to the file itself. It used to produce nothing at all.
func TestMinimalApiProgramIsAModuleUnit(t *testing.T) {
	byID, raw := analysed(t, "src/Shop.Api/Program.cs", `using Microsoft.EntityFrameworkCore;
using Shop.Api.Data;
using Shop.Api.Services;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddDbContext<ShopDbContext>();
builder.Services.AddScoped<IOrderService, OrderService>();

var app = builder.Build();
app.MapGet("/orders/{id}", (int id, IOrderService orders) => orders.Get(id));
app.Run();
`)
	require.Len(t, byID, 1, "the file is one unit: itself")
	program := byID["src/Shop.Api/Program#"]
	require.NotNil(t, program)
	assert.Equal(t, unit.KindModule, program.Kind)
	assert.Equal(t, "Program", program.Name)
	assert.Empty(t, program.Markers)
	assert.Contains(t, program.Refs, unit.Ref{Module: "Shop.Api.Data", Name: "ShopDbContext"})
	assert.Contains(t, program.Refs, unit.Ref{Module: "Shop.Api.Services", Name: "IOrderService"})
	assert.Contains(t, program.Refs, unit.Ref{Module: "Shop.Api.Services", Name: "OrderService"})
	assert.Contains(t, raw, "Microsoft.EntityFrameworkCore")

	// A file with types and no top-level code has no module unit.
	types, _ := analysed(t, "src/Shop.Api/Dtos/Empty.cs", "namespace Shop.Api.Dtos;\n\npublic class Empty { }\n")
	assert.NotContains(t, types, "src/Shop.Api/Dtos/Empty#")
}

// A test class's `[Fact]` methods mark the test class, and nothing else.
func TestTestProjects(t *testing.T) {
	byID, _ := analysed(t, "tests/Shop.Api.Tests/OrdersControllerTests.cs", `using Shop.Api.Controllers;
using Xunit;

namespace Shop.Api.Tests;

public class OrdersControllerTests
{
    [Fact]
    public void Gets_an_order()
    {
        var sut = new OrdersController(null);
    }
}

public class Fixture
{
}
`)
	tests := byID["Shop.Api.Tests.OrdersControllerTests"]
	require.NotNil(t, tests)
	assert.Equal(t, []unit.Marker{attribute("Fact")}, tests.Markers)
	assert.Contains(t, tests.Refs, unit.Ref{Module: "Shop.Api.Controllers", Name: "OrdersController"})
	assert.Empty(t, byID["Shop.Api.Tests.Fixture"].Markers)
}

// A nested type's attribute is the nested type's; the enclosing one keeps
// only its own.
func TestNestedTypeAttributesStayWithTheNestedType(t *testing.T) {
	byID, _ := analysed(t, "src/Shop.Api/Domain/Settings.cs", `namespace Shop.Api.Domain;

[Serializable]
public class Settings
{
    [Obsolete]
    public class Legacy
    {
    }

    [Required]
    public string Name { get; set; }
}
`)
	assert.ElementsMatch(t, []unit.Marker{attribute("Serializable"), attribute("Required")}, byID["Shop.Api.Domain.Settings"].Markers)
	legacy := byID["Shop.Api.Domain.Settings.Legacy"]
	require.NotNil(t, legacy)
	assert.Equal(t, []unit.Marker{attribute("Obsolete")}, legacy.Markers)
	assert.Equal(t, "Shop.Api.Domain.Settings", legacy.Owner)
}
