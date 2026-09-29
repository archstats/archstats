package java

import (
	"sort"
	"strings"
	"testing"

	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mini-applications on the JVM server frameworks, read the way the engine
// reads a file: the units, their markers and references are what the UI's
// framework profiles sort into lanes, so what is asserted here is exactly
// what layers.jvm.test.ts feeds them.

type javaFile struct{ path, src string }

type analysed struct {
	units   map[string]*unit.Unit
	imports map[string][]string // file -> raw imports
}

func analyse(t *testing.T, files ...javaFile) analysed {
	t.Helper()
	analyzer := &javaAnalyzer{lp: (&Extension{}).createJavaLanguagePack()}
	out := analysed{units: map[string]*unit.Unit{}, imports: map[string][]string{}}
	for _, f := range files {
		res := analyzer.AnalyzeFile(&mockFile{path: f.path, content: []byte(f.src)})
		require.NotNilf(t, res, "%s was not analysed", f.path)
		for _, u := range res.Units {
			out.units[u.ID] = u
		}
		for _, s := range res.Snippets {
			if s.Type == "modularity__import__raw" {
				out.imports[f.path] = append(out.imports[f.path], s.Value)
			}
		}
	}
	return out
}

func (a analysed) unit(t *testing.T, id string) *unit.Unit {
	t.Helper()
	u, ok := a.units[id]
	require.Truef(t, ok, "%s is not a unit; have %v", id, a.ids())
	return u
}

func (a analysed) ids() []string {
	var ids []string
	for id := range a.units {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// "annotation:Service", "supertype:JpaRepository": a marker as the UI reads it.
func markersOf(u *unit.Unit) []string {
	var out []string
	for _, m := range u.Markers {
		out = append(out, m.Source+":"+m.Key)
	}
	return out
}

func assertMarkers(t *testing.T, u *unit.Unit, want ...string) {
	t.Helper()
	assert.ElementsMatchf(t, want, markersOf(u), "markers of %s", u.ID)
}

func hasRef(u *unit.Unit, module, name string) bool {
	for _, r := range u.Refs {
		if r.Module == module && r.Name == name {
			return true
		}
	}
	return false
}

func assertRef(t *testing.T, u *unit.Unit, module, name string) {
	t.Helper()
	assert.Truef(t, hasRef(u, module, name), "%s should reference %s.%s; has %v", u.ID, module, name, u.Refs)
}

// ── Spring ──────────────────────────────────────────────────────────────────

var spring = []javaFile{
	{"src/main/java/com/acme/shop/web/OrderController.java", `package com.acme.shop.web;

import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import com.acme.shop.service.OrderService;
import com.acme.shop.model.Order;

@RestController
@RequestMapping("/orders")
public class OrderController {
    private final OrderService service;
    public OrderController(OrderService service) { this.service = service; }
    @GetMapping("/{id}") public OrderDto get(long id) { return OrderDto.of(service.find(id)); }
}
`},
	{"src/main/java/com/acme/shop/web/OrderDto.java", `package com.acme.shop.web;

import com.acme.shop.model.Order;

public record OrderDto(long id, String state) {
    static OrderDto of(Order o) { return new OrderDto(o.getId(), o.getState().name()); }
}
`},
	{"src/main/java/com/acme/shop/service/OrderService.java", `package com.acme.shop.service;

import org.springframework.transaction.annotation.Transactional;
import org.springframework.jdbc.core.RowMapper;
import com.acme.shop.repo.OrderRepository;
import com.acme.shop.model.Order;

@org.springframework.stereotype.Service
@Transactional(readOnly = true)
public class OrderService {
    private final OrderRepository repo;
    public OrderService(OrderRepository repo) { this.repo = repo; }
    public Order find(long id) { return repo.findById(id).orElseThrow(); }

    private static final class OrderMapper implements RowMapper<Order> {
        public Order mapRow(java.sql.ResultSet rs, int i) { return new Order(); }
    }
}
`},
	{"src/main/java/com/acme/shop/repo/OrderRepository.java", `package com.acme.shop.repo;

import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;
import com.acme.shop.model.Order;

public interface OrderRepository extends JpaRepository<Order, Long>, JpaSpecificationExecutor<Order> {
}
`},
	{"src/main/java/com/acme/shop/model/Order.java", `package com.acme.shop.model;

import jakarta.persistence.Entity;
import jakarta.persistence.Table;
import jakarta.persistence.Embedded;

@Entity
@Table(name = "orders")
public class Order extends Auditable {
    private Long id;
    @Embedded private Money total;
    private OrderState state;
    public Long getId() { return id; }
    public OrderState getState() { return state; }

    public enum OrderState { NEW, PAID }
}
`},
	{"src/main/java/com/acme/shop/model/Auditable.java", `package com.acme.shop.model;

import jakarta.persistence.MappedSuperclass;

@MappedSuperclass
public abstract class Auditable { private java.time.Instant created; }
`},
	{"src/main/java/com/acme/shop/model/Money.java", `package com.acme.shop.model;

import jakarta.persistence.Embeddable;

@Embeddable
public class Money { private long cents; }
`},
	{"src/main/java/com/acme/shop/ShopApplication.java", `package com.acme.shop;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class ShopApplication {
    public static void main(String[] args) { SpringApplication.run(ShopApplication.class, args); }
}
`},
	{"src/main/java/com/acme/shop/config/WebConfig.java", `package com.acme.shop.config;

import org.springframework.context.annotation.Configuration;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;
import com.acme.shop.web.OrderController;

@Configuration
public class WebConfig implements WebMvcConfigurer {
}
`},
	{"src/main/java/com/acme/shop/messaging/OrderEvents.java", `package com.acme.shop.messaging;

import org.springframework.stereotype.Component;
import org.springframework.kafka.annotation.KafkaListener;
import com.acme.shop.service.OrderService;

@Component
public class OrderEvents {
    private final OrderService service;
    public OrderEvents(OrderService service) { this.service = service; }
    @KafkaListener(topics = "orders") public void on(String message) { service.find(1L); }
}
`},
	{"src/test/java/com/acme/shop/service/OrderServiceTest.java", `package com.acme.shop.service;

import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.context.TestConfiguration;

@SpringBootTest
class OrderServiceTest {
    @TestConfiguration
    static class Beans {}
}
`},
}

func TestSpringUnitsCarryTheirOwnMarkers(t *testing.T) {
	a := analyse(t, spring...)

	assertMarkers(t, a.unit(t, "com.acme.shop.web.OrderController"), "annotation:RestController", "annotation:RequestMapping")
	// A qualified stereotype is the stereotype; it used to be read as no annotation at all.
	assertMarkers(t, a.unit(t, "com.acme.shop.service.OrderService"), "annotation:Service", "annotation:Transactional")
	assertMarkers(t, a.unit(t, "com.acme.shop.repo.OrderRepository"), "supertype:JpaRepository", "supertype:JpaSpecificationExecutor", "supertype:interface")
	assertMarkers(t, a.unit(t, "com.acme.shop.model.Order"), "annotation:Entity", "annotation:Table", "supertype:Auditable")
	assertMarkers(t, a.unit(t, "com.acme.shop.model.Auditable"), "annotation:MappedSuperclass")
	assertMarkers(t, a.unit(t, "com.acme.shop.model.Money"), "annotation:Embeddable")
	assertMarkers(t, a.unit(t, "com.acme.shop.ShopApplication"), "annotation:SpringBootApplication")
	assertMarkers(t, a.unit(t, "com.acme.shop.config.WebConfig"), "annotation:Configuration", "supertype:WebMvcConfigurer")
	// A record is a type with no framework markers; the DTO is told apart from the entity by them.
	assertMarkers(t, a.unit(t, "com.acme.shop.web.OrderDto"))
	assertMarkers(t, a.unit(t, "com.acme.shop.messaging.OrderEvents"), "annotation:Component")
	for _, u := range a.units {
		assert.Equal(t, unit.KindType, u.Kind, u.ID)
	}
}

// fineract writes its JDBC row mappers as nested classes of the service that
// uses them, and 43 of its @Service classes "implemented RowMapper". The
// nested class is its own unit, owned by the one around it, and the owner
// carries nothing of it.
func TestNestedClassesAreOwnedUnitsWithTheirOwnMarkers(t *testing.T) {
	a := analyse(t, spring...)

	mapper := a.unit(t, "com.acme.shop.service.OrderService.OrderMapper")
	assert.Equal(t, "com.acme.shop.service.OrderService", mapper.Owner)
	assert.Equal(t, "OrderMapper", mapper.Name)
	assertMarkers(t, mapper, "supertype:RowMapper")
	assert.Empty(t, a.unit(t, "com.acme.shop.service.OrderService").Owner)

	state := a.unit(t, "com.acme.shop.model.Order.OrderState")
	assert.Equal(t, "com.acme.shop.model.Order", state.Owner)
	assertMarkers(t, state)

	// A test's nested @TestConfiguration does not make the test a configuration.
	assertMarkers(t, a.unit(t, "com.acme.shop.service.OrderServiceTest"), "annotation:SpringBootTest")
	assertMarkers(t, a.unit(t, "com.acme.shop.service.OrderServiceTest.Beans"), "annotation:TestConfiguration")
}

func TestSpringReferencesRunBetweenLayers(t *testing.T) {
	a := analyse(t, spring...)

	assertRef(t, a.unit(t, "com.acme.shop.web.OrderController"), "com.acme.shop.service", "OrderService")
	assertRef(t, a.unit(t, "com.acme.shop.web.OrderController"), "com.acme.shop.model", "Order")
	assertRef(t, a.unit(t, "com.acme.shop.service.OrderService"), "com.acme.shop.repo", "OrderRepository")
	assertRef(t, a.unit(t, "com.acme.shop.repo.OrderRepository"), "com.acme.shop.model", "Order")
	assertRef(t, a.unit(t, "com.acme.shop.messaging.OrderEvents"), "com.acme.shop.service", "OrderService")
	// Same-package supertype, resolved exactly there.
	assert.Contains(t, a.unit(t, "com.acme.shop.model.Order").Refs, unit.Ref{Module: "com.acme.shop.model", Name: "Auditable", Exact: true})
	// A configuration reaching up into the web layer is a reference like any other; whether it is a violation is the profile's call.
	assertRef(t, a.unit(t, "com.acme.shop.config.WebConfig"), "com.acme.shop.web", "OrderController")
	// The file's references stay with its public type; the nested class carries none of its own.
	assert.Empty(t, a.unit(t, "com.acme.shop.service.OrderService.OrderMapper").Refs)
}

func TestSpringRawImportsAreTheDetectionEvidence(t *testing.T) {
	a := analyse(t, spring...)
	assert.Contains(t, a.imports["src/main/java/com/acme/shop/web/OrderController.java"], "org.springframework.web.bind.annotation.RestController")
	assert.Contains(t, a.imports["src/main/java/com/acme/shop/repo/OrderRepository.java"], "org.springframework.data.jpa.repository.JpaRepository")
	assert.Contains(t, a.imports["src/main/java/com/acme/shop/messaging/OrderEvents.java"], "org.springframework.kafka.annotation.KafkaListener")
	// The qualified annotation leaves no import behind: the marker is the only evidence of the stereotype.
	for _, imp := range a.imports["src/main/java/com/acme/shop/service/OrderService.java"] {
		assert.False(t, strings.HasSuffix(imp, ".Service"))
	}
}

// A second top-level type in a file is a type: it used to vanish, and its
// annotations landed on the public one.
func TestASecondTopLevelTypeIsItsOwnUnit(t *testing.T) {
	a := analyse(t, javaFile{"src/com/acme/Jobs.java", `package com.acme;

import org.springframework.stereotype.Service;
import org.springframework.context.annotation.Configuration;

@Service
public class Jobs {}

@Configuration
class JobsConfig {}
`})
	assertMarkers(t, a.unit(t, "com.acme.Jobs"), "annotation:Service")
	cfg := a.unit(t, "com.acme.JobsConfig")
	assert.Empty(t, cfg.Owner)
	assertMarkers(t, cfg, "annotation:Configuration")
}

// ── Jakarta EE ──────────────────────────────────────────────────────────────

var jakarta = []javaFile{
	{"src/main/java/com/acme/bank/api/AccountResource.java", `package com.acme.bank.api;

import jakarta.ws.rs.Path;
import jakarta.ws.rs.GET;
import jakarta.enterprise.context.RequestScoped;
import jakarta.inject.Inject;
import com.acme.bank.service.AccountService;

@Path("/accounts")
@RequestScoped
public class AccountResource {
    @Inject AccountService service;
    @GET public String list() { return service.list().toString(); }
}
`},
	{"src/main/java/com/acme/bank/api/BankApplication.java", `package com.acme.bank.api;

import jakarta.ws.rs.ApplicationPath;
import jakarta.ws.rs.core.Application;

@ApplicationPath("/api")
public class BankApplication extends Application {}
`},
	{"src/main/java/com/acme/bank/service/AccountService.java", `package com.acme.bank.service;

import jakarta.ejb.Stateless;
import jakarta.inject.Inject;
import com.acme.bank.dao.AccountDao;
import com.acme.bank.model.Account;

@Stateless
public class AccountService {
    @Inject AccountDao dao;
    public java.util.List<Account> list() { return dao.all(); }
}
`},
	{"src/main/java/com/acme/bank/dao/AccountDao.java", `package com.acme.bank.dao;

import jakarta.enterprise.context.ApplicationScoped;
import jakarta.persistence.EntityManager;
import jakarta.persistence.PersistenceContext;
import com.acme.bank.model.Account;

@ApplicationScoped
public class AccountDao {
    @PersistenceContext EntityManager em;
    public java.util.List<Account> all() { return em.createQuery("select a from Account a", Account.class).getResultList(); }
}
`},
	{"src/main/java/com/acme/bank/model/Account.java", `package com.acme.bank.model;

import jakarta.persistence.Entity;

@Entity
public class Account { private Long id; }
`},
	{"src/main/java/com/acme/bank/messaging/TransferListener.java", `package com.acme.bank.messaging;

import jakarta.ejb.MessageDriven;
import jakarta.jms.MessageListener;
import jakarta.jms.Message;
import com.acme.bank.service.AccountService;

@MessageDriven(mappedName = "jms/transfers")
public class TransferListener implements MessageListener {
    public void onMessage(Message m) { new AccountService().list(); }
}
`},
	{"src/main/java/com/acme/bank/web/AuditFilter.java", `package com.acme.bank.web;

import jakarta.servlet.Filter;
import jakarta.servlet.annotation.WebFilter;

@WebFilter("/*")
public class AuditFilter implements Filter {}
`},
}

func TestJakartaUnits(t *testing.T) {
	a := analyse(t, jakarta...)
	assertMarkers(t, a.unit(t, "com.acme.bank.api.AccountResource"), "annotation:Path", "annotation:RequestScoped")
	assertMarkers(t, a.unit(t, "com.acme.bank.api.BankApplication"), "annotation:ApplicationPath", "supertype:Application")
	assertMarkers(t, a.unit(t, "com.acme.bank.service.AccountService"), "annotation:Stateless")
	// The DAO's evidence is what it imports, not what it is annotated with.
	assertMarkers(t, a.unit(t, "com.acme.bank.dao.AccountDao"), "annotation:ApplicationScoped")
	assert.Contains(t, a.imports["src/main/java/com/acme/bank/dao/AccountDao.java"], "jakarta.persistence.EntityManager")
	assertMarkers(t, a.unit(t, "com.acme.bank.model.Account"), "annotation:Entity")
	assertMarkers(t, a.unit(t, "com.acme.bank.messaging.TransferListener"), "annotation:MessageDriven", "supertype:MessageListener")
	assertMarkers(t, a.unit(t, "com.acme.bank.web.AuditFilter"), "annotation:WebFilter", "supertype:Filter")

	assertRef(t, a.unit(t, "com.acme.bank.api.AccountResource"), "com.acme.bank.service", "AccountService")
	assertRef(t, a.unit(t, "com.acme.bank.service.AccountService"), "com.acme.bank.dao", "AccountDao")
	assertRef(t, a.unit(t, "com.acme.bank.dao.AccountDao"), "com.acme.bank.model", "Account")
}

// ── Quarkus ─────────────────────────────────────────────────────────────────

var quarkus = []javaFile{
	{"src/main/java/com/acme/inventory/ItemResource.java", `package com.acme.inventory;

import jakarta.ws.rs.Path;
import jakarta.ws.rs.GET;
import jakarta.inject.Inject;

@Path("/items")
public class ItemResource {
    @Inject ItemService service;
    @GET public java.util.List<Item> all() { return service.all(); }
}
`},
	{"src/main/java/com/acme/inventory/ItemService.java", `package com.acme.inventory;

import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import io.quarkus.logging.Log;

@ApplicationScoped
public class ItemService {
    @Inject ItemRepository repo;
    public java.util.List<Item> all() { Log.info("all"); return repo.listAll(); }
}
`},
	{"src/main/java/com/acme/inventory/ItemRepository.java", `package com.acme.inventory;

import jakarta.enterprise.context.ApplicationScoped;
import io.quarkus.hibernate.orm.panache.PanacheRepository;

@ApplicationScoped
public class ItemRepository implements PanacheRepository<Item> {}
`},
	{"src/main/java/com/acme/inventory/Item.java", `package com.acme.inventory;

import jakarta.persistence.Entity;
import io.quarkus.hibernate.orm.panache.PanacheEntity;

@Entity
public class Item extends PanacheEntity { public String sku; }
`},
	{"src/main/java/com/acme/inventory/StockConsumer.java", `package com.acme.inventory;

import jakarta.enterprise.context.ApplicationScoped;
import org.eclipse.microprofile.reactive.messaging.Incoming;

@ApplicationScoped
public class StockConsumer {
    @Incoming("stock") public void on(String sku) {}
}
`},
	{"src/main/java/com/acme/inventory/SupplierClient.java", `package com.acme.inventory;

import jakarta.ws.rs.Path;
import jakarta.ws.rs.GET;
import org.eclipse.microprofile.rest.client.inject.RegisterRestClient;

@RegisterRestClient(configKey = "suppliers")
@Path("/suppliers")
public interface SupplierClient {
    @GET java.util.List<String> all();
}
`},
	{"src/main/java/com/acme/inventory/Main.java", `package com.acme.inventory;

import io.quarkus.runtime.Quarkus;
import io.quarkus.runtime.annotations.QuarkusMain;

@QuarkusMain
public class Main { public static void main(String... a) { Quarkus.run(a); } }
`},
}

func TestQuarkusUnits(t *testing.T) {
	a := analyse(t, quarkus...)
	assertMarkers(t, a.unit(t, "com.acme.inventory.ItemResource"), "annotation:Path")
	assertMarkers(t, a.unit(t, "com.acme.inventory.ItemService"), "annotation:ApplicationScoped")
	// A Panache repository is a scoped bean and a repository; the supertype is what makes it the latter.
	assertMarkers(t, a.unit(t, "com.acme.inventory.ItemRepository"), "annotation:ApplicationScoped", "supertype:PanacheRepository")
	assertMarkers(t, a.unit(t, "com.acme.inventory.Item"), "annotation:Entity", "supertype:PanacheEntity")
	assertMarkers(t, a.unit(t, "com.acme.inventory.StockConsumer"), "annotation:ApplicationScoped")
	assert.Contains(t, a.imports["src/main/java/com/acme/inventory/StockConsumer.java"], "org.eclipse.microprofile.reactive.messaging.Incoming")
	// A REST client interface carries @Path too: the client annotation is what tells it from a resource.
	assertMarkers(t, a.unit(t, "com.acme.inventory.SupplierClient"), "annotation:RegisterRestClient", "annotation:Path", "supertype:interface")
	assertMarkers(t, a.unit(t, "com.acme.inventory.Main"), "annotation:QuarkusMain")

	assertRef(t, a.unit(t, "com.acme.inventory.ItemResource"), "com.acme.inventory", "ItemService")
	assertRef(t, a.unit(t, "com.acme.inventory.ItemService"), "com.acme.inventory", "ItemRepository")
	assertRef(t, a.unit(t, "com.acme.inventory.ItemRepository"), "com.acme.inventory", "Item")
}

// ── Micronaut ───────────────────────────────────────────────────────────────

var micronaut = []javaFile{
	{"src/main/java/com/acme/pay/PaymentController.java", `package com.acme.pay;

import io.micronaut.http.annotation.Controller;
import io.micronaut.http.annotation.Get;

@Controller("/payments")
public class PaymentController {
    private final PaymentService service;
    PaymentController(PaymentService service) { this.service = service; }
    @Get public java.util.List<Payment> all() { return service.all(); }
}
`},
	{"src/main/java/com/acme/pay/PaymentService.java", `package com.acme.pay;

import jakarta.inject.Singleton;

@Singleton
public class PaymentService {
    private final PaymentRepository repo;
    private final BankClient bank;
    PaymentService(PaymentRepository repo, BankClient bank) { this.repo = repo; this.bank = bank; }
    public java.util.List<Payment> all() { return repo.findAll(); }
}
`},
	{"src/main/java/com/acme/pay/PaymentRepository.java", `package com.acme.pay;

import io.micronaut.data.jdbc.annotation.JdbcRepository;
import io.micronaut.data.model.query.builder.sql.Dialect;
import io.micronaut.data.repository.CrudRepository;

@JdbcRepository(dialect = Dialect.POSTGRES)
public interface PaymentRepository extends CrudRepository<Payment, Long> {}
`},
	{"src/main/java/com/acme/pay/Payment.java", `package com.acme.pay;

import io.micronaut.data.annotation.MappedEntity;
import io.micronaut.data.annotation.Id;

@MappedEntity
public class Payment { @Id private Long id; }
`},
	{"src/main/java/com/acme/pay/BankClient.java", `package com.acme.pay;

import io.micronaut.http.client.annotation.Client;
import io.micronaut.http.annotation.Get;

@Client("/bank")
public interface BankClient { @Get("/balance") long balance(); }
`},
	{"src/main/java/com/acme/pay/PaymentListener.java", `package com.acme.pay;

import io.micronaut.configuration.kafka.annotation.KafkaListener;
import io.micronaut.configuration.kafka.annotation.Topic;

@KafkaListener(groupId = "pay")
public class PaymentListener {
    @Topic("payments") void on(String key) {}
}
`},
	{"src/main/java/com/acme/pay/PayConfig.java", `package com.acme.pay;

import io.micronaut.context.annotation.ConfigurationProperties;

@ConfigurationProperties("pay")
public class PayConfig { private String currency; }
`},
	{"src/main/java/com/acme/pay/PayFactory.java", `package com.acme.pay;

import io.micronaut.context.annotation.Factory;
import io.micronaut.context.annotation.Bean;

@Factory
public class PayFactory { @Bean java.time.Clock clock() { return java.time.Clock.systemUTC(); } }
`},
}

func TestMicronautUnits(t *testing.T) {
	a := analyse(t, micronaut...)
	assertMarkers(t, a.unit(t, "com.acme.pay.PaymentController"), "annotation:Controller")
	assertMarkers(t, a.unit(t, "com.acme.pay.PaymentService"), "annotation:Singleton")
	// Generic type arguments are not part of the supertype's name.
	assertMarkers(t, a.unit(t, "com.acme.pay.PaymentRepository"), "annotation:JdbcRepository", "supertype:CrudRepository", "supertype:interface")
	assertMarkers(t, a.unit(t, "com.acme.pay.Payment"), "annotation:MappedEntity")
	assertMarkers(t, a.unit(t, "com.acme.pay.BankClient"), "annotation:Client", "supertype:interface")
	assertMarkers(t, a.unit(t, "com.acme.pay.PaymentListener"), "annotation:KafkaListener")
	assertMarkers(t, a.unit(t, "com.acme.pay.PayConfig"), "annotation:ConfigurationProperties")
	assertMarkers(t, a.unit(t, "com.acme.pay.PayFactory"), "annotation:Factory")

	// Everything is in one package here, and the references are resolved exactly in it.
	svc := a.unit(t, "com.acme.pay.PaymentService")
	assert.Contains(t, svc.Refs, unit.Ref{Module: "com.acme.pay", Name: "PaymentRepository", Exact: true})
	assert.Contains(t, svc.Refs, unit.Ref{Module: "com.acme.pay", Name: "BankClient", Exact: true})
	assert.Contains(t, a.unit(t, "com.acme.pay.PaymentController").Refs, unit.Ref{Module: "com.acme.pay", Name: "PaymentService", Exact: true})
}

// ── Vert.x ──────────────────────────────────────────────────────────────────

var vertx = []javaFile{
	{"src/main/java/com/acme/chat/MainVerticle.java", `package com.acme.chat;

import io.vertx.core.AbstractVerticle;
import io.vertx.core.Promise;
import io.vertx.ext.web.Router;

public class MainVerticle extends AbstractVerticle {
    @Override public void start(Promise<Void> p) {
        Router r = Router.router(vertx);
        r.get("/messages").handler(new MessageHandler(new MessageStore(vertx)));
    }
}
`},
	{"src/main/java/com/acme/chat/MessageHandler.java", `package com.acme.chat;

import io.vertx.core.Handler;
import io.vertx.ext.web.RoutingContext;

public class MessageHandler implements Handler<RoutingContext> {
    private final MessageStore store;
    MessageHandler(MessageStore store) { this.store = store; }
    public void handle(RoutingContext ctx) { store.all().onSuccess(ctx::json); }
}
`},
	{"src/main/java/com/acme/chat/MessageStore.java", `package com.acme.chat;

import io.vertx.core.Vertx;
import io.vertx.core.Future;
import io.vertx.pgclient.PgPool;

public class MessageStore {
    private final PgPool pool;
    MessageStore(Vertx vertx) { this.pool = PgPool.pool(vertx); }
    Future<java.util.List<ChatMessage>> all() { return Future.succeededFuture(java.util.List.of()); }
}
`},
	{"src/main/java/com/acme/chat/Broadcaster.java", `package com.acme.chat;

import io.vertx.core.eventbus.EventBus;

public class Broadcaster {
    private final EventBus bus;
    Broadcaster(EventBus bus) { this.bus = bus; }
    void send(ChatMessage m) { bus.publish("chat", m); }
}
`},
	{"src/main/java/com/acme/chat/ChatMessage.java", `package com.acme.chat;

import io.vertx.codegen.annotations.DataObject;
import io.vertx.core.json.JsonObject;

@DataObject
public class ChatMessage {
    private String text;
    public ChatMessage(JsonObject json) { text = json.getString("text"); }
    public JsonObject toJson() { return new JsonObject().put("text", text); }
}
`},
	{"src/main/java/com/acme/chat/ChatMessageCodec.java", `package com.acme.chat;

import io.vertx.core.buffer.Buffer;
import io.vertx.core.eventbus.MessageCodec;

public class ChatMessageCodec implements MessageCodec<ChatMessage, ChatMessage> {
    public void encodeToWire(Buffer b, ChatMessage m) {}
}
`},
}

func TestVertxUnits(t *testing.T) {
	a := analyse(t, vertx...)
	assertMarkers(t, a.unit(t, "com.acme.chat.MainVerticle"), "supertype:AbstractVerticle")
	assertMarkers(t, a.unit(t, "com.acme.chat.MessageHandler"), "supertype:Handler")
	// A store and a broadcaster are told apart by what they import alone.
	assertMarkers(t, a.unit(t, "com.acme.chat.MessageStore"))
	assert.Contains(t, a.imports["src/main/java/com/acme/chat/MessageStore.java"], "io.vertx.pgclient.PgPool")
	assertMarkers(t, a.unit(t, "com.acme.chat.Broadcaster"))
	assert.Contains(t, a.imports["src/main/java/com/acme/chat/Broadcaster.java"], "io.vertx.core.eventbus.EventBus")
	assertMarkers(t, a.unit(t, "com.acme.chat.ChatMessage"), "annotation:DataObject")
	assertMarkers(t, a.unit(t, "com.acme.chat.ChatMessageCodec"), "supertype:MessageCodec")

	assert.Contains(t, a.unit(t, "com.acme.chat.MainVerticle").Refs, unit.Ref{Module: "com.acme.chat", Name: "MessageHandler", Exact: true})
	assert.Contains(t, a.unit(t, "com.acme.chat.MessageHandler").Refs, unit.Ref{Module: "com.acme.chat", Name: "MessageStore", Exact: true})
	assert.Contains(t, a.unit(t, "com.acme.chat.MessageStore").Refs, unit.Ref{Module: "com.acme.chat", Name: "ChatMessage", Exact: true})
}

// ── Dropwizard ──────────────────────────────────────────────────────────────

var dropwizard = []javaFile{
	{"src/main/java/com/acme/books/BooksApplication.java", `package com.acme.books;

import io.dropwizard.core.Application;
import io.dropwizard.core.setup.Environment;
import com.acme.books.resources.BookResource;
import com.acme.books.db.BookDAO;

public class BooksApplication extends Application<BooksConfiguration> {
    public static void main(String[] args) throws Exception { new BooksApplication().run(args); }
    @Override public void run(BooksConfiguration c, Environment e) { e.jersey().register(new BookResource(new BookDAO(null))); }
}
`},
	{"src/main/java/com/acme/books/BooksConfiguration.java", `package com.acme.books;

import io.dropwizard.core.Configuration;
import io.dropwizard.db.DataSourceFactory;

public class BooksConfiguration extends Configuration {
    private DataSourceFactory database = new DataSourceFactory();
}
`},
	{"src/main/java/com/acme/books/resources/BookResource.java", `package com.acme.books.resources;

import jakarta.ws.rs.Path;
import jakarta.ws.rs.GET;
import io.dropwizard.hibernate.UnitOfWork;
import com.acme.books.db.BookDAO;
import com.acme.books.core.Book;

@Path("/books")
public class BookResource {
    private final BookDAO dao;
    public BookResource(BookDAO dao) { this.dao = dao; }
    @GET @UnitOfWork public java.util.List<Book> all() { return dao.findAll(); }
}
`},
	{"src/main/java/com/acme/books/db/BookDAO.java", `package com.acme.books.db;

import io.dropwizard.hibernate.AbstractDAO;
import org.hibernate.SessionFactory;
import com.acme.books.core.Book;

public class BookDAO extends AbstractDAO<Book> {
    public BookDAO(SessionFactory f) { super(f); }
    public java.util.List<Book> findAll() { return list(query("from Book")); }
}
`},
	{"src/main/java/com/acme/books/core/Book.java", `package com.acme.books.core;

import jakarta.persistence.Entity;

@Entity
public class Book { private long id; }
`},
	{"src/main/java/com/acme/books/health/BooksHealthCheck.java", `package com.acme.books.health;

import com.codahale.metrics.health.HealthCheck;

public class BooksHealthCheck extends HealthCheck {
    @Override protected Result check() { return Result.healthy(); }
}
`},
}

func TestDropwizardUnits(t *testing.T) {
	a := analyse(t, dropwizard...)
	assertMarkers(t, a.unit(t, "com.acme.books.BooksApplication"), "supertype:Application")
	assertMarkers(t, a.unit(t, "com.acme.books.BooksConfiguration"), "supertype:Configuration")
	assertMarkers(t, a.unit(t, "com.acme.books.resources.BookResource"), "annotation:Path")
	assertMarkers(t, a.unit(t, "com.acme.books.db.BookDAO"), "supertype:AbstractDAO")
	assertMarkers(t, a.unit(t, "com.acme.books.core.Book"), "annotation:Entity")
	assertMarkers(t, a.unit(t, "com.acme.books.health.BooksHealthCheck"), "supertype:HealthCheck")

	// The application wires the resources and the DAOs: it reaches every layer.
	assertRef(t, a.unit(t, "com.acme.books.BooksApplication"), "com.acme.books.resources", "BookResource")
	assertRef(t, a.unit(t, "com.acme.books.BooksApplication"), "com.acme.books.db", "BookDAO")
	assertRef(t, a.unit(t, "com.acme.books.resources.BookResource"), "com.acme.books.db", "BookDAO")
	assertRef(t, a.unit(t, "com.acme.books.db.BookDAO"), "com.acme.books.core", "Book")
}

// ── Apache Beam ─────────────────────────────────────────────────────────────

var beam = []javaFile{
	{"src/main/java/com/acme/etl/OrdersPipeline.java", `package com.acme.etl;

import org.apache.beam.sdk.Pipeline;
import org.apache.beam.sdk.io.TextIO;
import org.apache.beam.sdk.options.PipelineOptionsFactory;

public class OrdersPipeline {
    public static void main(String[] args) {
        OrdersOptions o = PipelineOptionsFactory.fromArgs(args).as(OrdersOptions.class);
        Pipeline p = Pipeline.create(o);
        p.apply(TextIO.read().from(o.getInput())).apply(new ParseOrders());
        p.run();
    }
}
`},
	{"src/main/java/com/acme/etl/OrdersOptions.java", `package com.acme.etl;

import org.apache.beam.sdk.options.PipelineOptions;

public interface OrdersOptions extends PipelineOptions {
    String getInput();
}
`},
	{"src/main/java/com/acme/etl/ParseOrders.java", `package com.acme.etl;

import org.apache.beam.sdk.transforms.PTransform;
import org.apache.beam.sdk.transforms.ParDo;
import org.apache.beam.sdk.transforms.DoFn;
import org.apache.beam.sdk.values.PCollection;

public class ParseOrders extends PTransform<PCollection<String>, PCollection<Order>> {
    @Override public PCollection<Order> expand(PCollection<String> in) { return in.apply(ParDo.of(new ParseFn())); }

    static class ParseFn extends DoFn<String, Order> {
        @ProcessElement public void process(@Element String line, OutputReceiver<Order> out) { out.output(new Order()); }
    }
}
`},
	{"src/main/java/com/acme/etl/TotalFn.java", `package com.acme.etl;

import org.apache.beam.sdk.transforms.Combine.CombineFn;

public class TotalFn extends CombineFn<Order, Long, Long> {}
`},
	{"src/main/java/com/acme/etl/OrderCoder.java", `package com.acme.etl;

import org.apache.beam.sdk.coders.CustomCoder;

public class OrderCoder extends CustomCoder<Order> {}
`},
	{"src/main/java/com/acme/etl/Order.java", `package com.acme.etl;

import org.apache.beam.sdk.coders.DefaultCoder;

@DefaultCoder(OrderCoder.class)
public class Order implements java.io.Serializable { long id; long total; }
`},
}

func TestBeamUnits(t *testing.T) {
	a := analyse(t, beam...)
	assertMarkers(t, a.unit(t, "com.acme.etl.OrdersPipeline"))
	assertMarkers(t, a.unit(t, "com.acme.etl.OrdersOptions"), "supertype:PipelineOptions", "supertype:interface")
	assertMarkers(t, a.unit(t, "com.acme.etl.ParseOrders"), "supertype:PTransform")
	// Beam's DoFns are usually nested in the transform that applies them. The
	// DoFn is a unit of its own, so the transform does not read as a DoFn.
	fn := a.unit(t, "com.acme.etl.ParseOrders.ParseFn")
	assert.Equal(t, "com.acme.etl.ParseOrders", fn.Owner)
	assertMarkers(t, fn, "supertype:DoFn")
	assertMarkers(t, a.unit(t, "com.acme.etl.TotalFn"), "supertype:CombineFn")
	assertMarkers(t, a.unit(t, "com.acme.etl.OrderCoder"), "supertype:CustomCoder")
	assertMarkers(t, a.unit(t, "com.acme.etl.Order"), "annotation:DefaultCoder", "supertype:Serializable")

	assert.Contains(t, a.imports["src/main/java/com/acme/etl/OrdersPipeline.java"], "org.apache.beam.sdk.io.TextIO")
	assert.Contains(t, a.unit(t, "com.acme.etl.OrdersPipeline").Refs, unit.Ref{Module: "com.acme.etl", Name: "ParseOrders", Exact: true})
	assert.Contains(t, a.unit(t, "com.acme.etl.ParseOrders").Refs, unit.Ref{Module: "com.acme.etl", Name: "Order", Exact: true})
}
