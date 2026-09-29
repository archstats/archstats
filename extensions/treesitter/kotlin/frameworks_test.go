package kotlin

import (
	"testing"

	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mini-applications on the Kotlin server frameworks, read the way the engine
// reads a file. What is asserted here is what layers.jvm.test.ts feeds the
// UI's framework profiles.

type ktFile struct{ path, src string }

func analyse(t *testing.T, files ...ktFile) (map[string]*unit.Unit, map[string][]string) {
	t.Helper()
	units := map[string]*unit.Unit{}
	imports := map[string][]string{}
	pack := createKotlinLanguagePack()
	for _, f := range files {
		res := pack.AnalyzeFileContent(f.path, []byte(f.src))
		require.NotNilf(t, res, "%s was not analysed", f.path)
		for _, u := range unitsFrom(f.path, res) {
			units[u.ID] = u
		}
		for _, s := range res.Snippets {
			if s.Type == "modularity__import__raw" {
				imports[f.path] = append(imports[f.path], s.Value)
			}
		}
	}
	return units, imports
}

func markersOf(u *unit.Unit) []string {
	var out []string
	for _, m := range u.Markers {
		out = append(out, m.Source+":"+m.Key)
	}
	return out
}

func assertMarkers(t *testing.T, units map[string]*unit.Unit, id string, want ...string) *unit.Unit {
	t.Helper()
	u, ok := units[id]
	require.Truef(t, ok, "%s is not a unit", id)
	assert.ElementsMatchf(t, want, markersOf(u), "markers of %s", id)
	return u
}

// ── Ktor ────────────────────────────────────────────────────────────────────

var ktor = []ktFile{
	{"src/main/kotlin/com/acme/shop/Application.kt", `package com.acme.shop

import io.ktor.server.application.Application
import io.ktor.server.application.install
import io.ktor.server.engine.embeddedServer
import io.ktor.server.netty.Netty
import com.acme.shop.routes.orderRoutes
import com.acme.shop.plugins.RequestTiming

fun main() {
    embeddedServer(Netty, port = 8080, module = Application::module).start(wait = true)
}

fun Application.module() {
    install(RequestTiming)
    orderRoutes()
}
`},
	{"src/main/kotlin/com/acme/shop/routes/OrderRoutes.kt", `package com.acme.shop.routes

import io.ktor.server.application.Application
import io.ktor.server.routing.Route
import io.ktor.server.routing.get
import io.ktor.server.routing.routing
import io.ktor.server.response.respond
import com.acme.shop.service.OrderService
import com.acme.shop.model.OrderDto

fun Application.orderRoutes() {
    routing { orders(OrderService()) }
}

fun Route.orders(service: OrderService) {
    get("/orders") { call.respond(service.all().map(OrderDto::from)) }
}
`},
	{"src/main/kotlin/com/acme/shop/plugins/RequestTiming.kt", `package com.acme.shop.plugins

import io.ktor.server.application.createApplicationPlugin

val RequestTiming = createApplicationPlugin(name = "RequestTiming") {
    onCall { call -> call.attributes.put(StartedAt, System.nanoTime()) }
}

class RateLimitPlugin(private val perMinute: Int) {
    fun allow(): Boolean = true
}
`},
	{"src/main/kotlin/com/acme/shop/service/OrderService.kt", `package com.acme.shop.service

import com.acme.shop.db.OrderRepository
import com.acme.shop.model.Order

class OrderService(private val repo: OrderRepository = OrderRepository()) {
    fun all(): List<Order> = repo.all()
}
`},
	{"src/main/kotlin/com/acme/shop/db/OrderRepository.kt", `package com.acme.shop.db

import org.jetbrains.exposed.sql.Table
import org.jetbrains.exposed.sql.selectAll
import org.jetbrains.exposed.sql.transactions.transaction
import com.acme.shop.model.Order

object Orders : Table("orders") {
    val id = long("id")
    val total = double("total")
}

class OrderRepository {
    fun all(): List<Order> = transaction { Orders.selectAll().map { Order(it[Orders.id], it[Orders.total]) } }
}
`},
	{"src/main/kotlin/com/acme/shop/model/Order.kt", `package com.acme.shop.model

import kotlinx.serialization.Serializable

data class Order(val id: Long, val total: Double)

@Serializable
data class OrderDto(val id: Long, val total: Double) {
    companion object {
        fun from(o: Order) = OrderDto(o.id, o.total)
    }
}
`},
}

// Ktor's entry points are extension functions: `fun Route.orders()` answers
// requests and `fun Application.module()` wires them. The receiver is the
// only thing that says so, and it is recorded as a marker the way a class's
// supertype is.
func TestKtorRoutesAreExtensionFunctionsMarkedByTheirReceiver(t *testing.T) {
	units, imports := analyse(t, ktor...)

	orders := assertMarkers(t, units, "com.acme.shop.routes.Route.orders", "receiver:Route")
	assert.Equal(t, unit.KindFunction, orders.Kind)
	assert.Equal(t, "orders", orders.Name)
	assert.Equal(t, "com.acme.shop.routes.Route", orders.Owner, "it belongs to Route, which is not declared in this codebase")
	assert.Contains(t, orders.Refs, unit.Ref{Module: "com.acme.shop.service", Name: "OrderService"})
	assert.Contains(t, orders.Refs, unit.Ref{Module: "com.acme.shop.model", Name: "OrderDto"})

	routes := assertMarkers(t, units, "com.acme.shop.routes.Application.orderRoutes", "receiver:Application")
	assert.Contains(t, routes.Refs, unit.Ref{Module: "com.acme.shop.routes", Name: "orders", Exact: true})

	module := assertMarkers(t, units, "com.acme.shop.Application.module", "receiver:Application")
	assert.Contains(t, module.Refs, unit.Ref{Module: "com.acme.shop.routes", Name: "orderRoutes"})
	assert.Contains(t, module.Refs, unit.Ref{Module: "com.acme.shop.plugins", Name: "RequestTiming"})

	// A plain top-level function has no receiver and no marker.
	main := assertMarkers(t, units, "com.acme.shop.main")
	assert.Empty(t, main.Owner)

	assert.Contains(t, imports["src/main/kotlin/com/acme/shop/routes/OrderRoutes.kt"], "io.ktor.server.routing.Route")
	assert.Contains(t, imports["src/main/kotlin/com/acme/shop/db/OrderRepository.kt"], "org.jetbrains.exposed.sql.Table")
}

func TestKtorDataAndModels(t *testing.T) {
	units, _ := analyse(t, ktor...)

	// An Exposed table is an object extending Table, written as a constructor call.
	assertMarkers(t, units, "com.acme.shop.db.Orders", "supertype:Table")
	repo := assertMarkers(t, units, "com.acme.shop.db.OrderRepository")
	assert.Equal(t, unit.KindType, repo.Kind)
	assert.Contains(t, units["com.acme.shop.db.OrderRepository.all"].Refs, unit.Ref{Module: "com.acme.shop.db", Name: "Orders", Exact: true})

	// Data classes say what they are with a keyword; the serializable DTO carries its annotation too.
	assertMarkers(t, units, "com.acme.shop.model.OrderDto", "annotation:Serializable", "keyword:data")
	assertMarkers(t, units, "com.acme.shop.model.Order", "keyword:data")
	// A companion object's function belongs to the class, not to a unit of its own.
	from, ok := units["com.acme.shop.model.OrderDto.from"]
	require.True(t, ok)
	assert.Equal(t, "com.acme.shop.model.OrderDto", from.Owner)
	assert.NotContains(t, units, "com.acme.shop.model.OrderDto.Companion")

	svc := units["com.acme.shop.service.OrderService"]
	require.NotNil(t, svc)
	assert.Contains(t, svc.Refs, unit.Ref{Module: "com.acme.shop.db", Name: "OrderRepository"})

	// A plugin declared as a top-level `val` is not a unit; only the class-shaped one is.
	assertMarkers(t, units, "com.acme.shop.plugins.RateLimitPlugin")
	assert.NotContains(t, units, "com.acme.shop.plugins.RequestTiming")
}

// ── Spring Boot in Kotlin ───────────────────────────────────────────────────

var springKotlin = []ktFile{
	{"src/main/kotlin/com/acme/shop/web/OrderController.kt", `package com.acme.shop.web

import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.RestController
import com.acme.shop.service.OrderService

@RestController
@RequestMapping("/orders")
class OrderController(private val service: OrderService) {
    @GetMapping("/{id}") fun get(id: Long) = service.find(id)
}
`},
	{"src/main/kotlin/com/acme/shop/service/OrderService.kt", `package com.acme.shop.service

import org.springframework.stereotype.Service
import org.springframework.transaction.annotation.Transactional
import com.acme.shop.repo.OrderRepository

@Service
@Transactional(readOnly = true)
class OrderService(private val repo: OrderRepository) {
    fun find(id: Long) = repo.findById(id).orElseThrow()
}
`},
	{"src/main/kotlin/com/acme/shop/repo/OrderRepository.kt", `package com.acme.shop.repo

import org.springframework.data.jpa.repository.JpaRepository
import com.acme.shop.model.Order

interface OrderRepository : JpaRepository<Order, Long>
`},
	{"src/main/kotlin/com/acme/shop/model/Order.kt", `package com.acme.shop.model

import jakarta.persistence.Entity
import jakarta.persistence.Id
import jakarta.persistence.Table

@Entity
@Table(name = "orders")
data class Order(@Id val id: Long = 0, val state: String = "NEW")
`},
	{"src/main/kotlin/com/acme/shop/ShopApplication.kt", `package com.acme.shop

import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.runApplication

@SpringBootApplication
class ShopApplication

fun main(args: Array<String>) {
    runApplication<ShopApplication>(*args)
}
`},
}

// Spring Boot is written in Kotlin as often as in Java now, with the same
// annotations on data classes and interfaces.
func TestSpringInKotlinCarriesTheSameMarkers(t *testing.T) {
	units, imports := analyse(t, springKotlin...)

	assertMarkers(t, units, "com.acme.shop.web.OrderController", "annotation:RestController", "annotation:RequestMapping")
	assertMarkers(t, units, "com.acme.shop.service.OrderService", "annotation:Service", "annotation:Transactional")
	// A generic supertype is keyed by its name alone, and an interface is a type.
	repo := assertMarkers(t, units, "com.acme.shop.repo.OrderRepository", "supertype:JpaRepository")
	assert.Equal(t, unit.KindType, repo.Kind)
	// A data class's constructor-property annotation (@Id) is the property's, not the class's.
	assertMarkers(t, units, "com.acme.shop.model.Order", "annotation:Entity", "annotation:Table", "keyword:data")
	assertMarkers(t, units, "com.acme.shop.ShopApplication", "annotation:SpringBootApplication")

	assert.Contains(t, units["com.acme.shop.web.OrderController"].Refs, unit.Ref{Module: "com.acme.shop.service", Name: "OrderService"})
	assert.Contains(t, units["com.acme.shop.service.OrderService"].Refs, unit.Ref{Module: "com.acme.shop.repo", Name: "OrderRepository"})
	assert.Contains(t, imports["src/main/kotlin/com/acme/shop/web/OrderController.kt"], "org.springframework.web.bind.annotation.RestController")
}
