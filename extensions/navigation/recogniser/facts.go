package recogniser

import (
	"regexp"
	"strings"
)

// Where the outside world gets into the code: an HTTP route, a page, a message
// or event consumed, a schedule, a command, a program's main. Architects think
// in use cases, and a use case starts at one of these. Every one is read from
// how a framework spells it; none is inferred from a name.
const (
	KindHTTP     = "http"
	KindPage     = "page"
	KindMessage  = "message"
	KindEvent    = "event"
	KindSchedule = "schedule"
	KindCLI      = "cli"
	KindMain     = "main"
	KindJob      = "job"
)

// An Entry is one entry point as found in a file, before it is linked to the
// unit and function that handle it.
type Entry struct {
	Kind      string
	Method    string
	Path      string
	Framework string
	// Handler is the handler as written where the route names it (a call's
	// argument, a urls.py view); empty for an annotated member, which is its
	// own handler.
	Handler string
	// Member and Class name the annotated declaration: the method and the
	// type it is in.
	Member string
	Class  string
	File   string
	Line   int
	// DeclLine is where the handler's declaration starts, for finding the
	// measured function that contains it.
	DeclLine int
	// Auth is what the route's marks, or its class's, say about who may
	// call it; Documented is "annotations" when they describe it for a
	// contract.
	Auth       string
	Documented string
	// Candidates are the frameworks a route read from where its file is may
	// belong to, in order; it counts for the first whose config is in the
	// workspace, and not at all if none is.
	Candidates []string
	// Mountable routes take the prefix their routing file is imported
	// under.
	Mountable bool
}

// What a class-level mark contributes to its members: the path prefix, and
// whether its members are a client's (Feign, Retrofit) rather than routes.
type ClassContext struct {
	Name   string
	Line   int
	Prefix string
	Client bool
	// Processor is a queue the class consumes.
	Processor string
	// Auth and Documented are the class's access and contract marks, which
	// its members inherit.
	Auth       string
	Documented bool
}

// A RouteImport is a routing file mounting another under a prefix.
type RouteImport struct {
	// Resource is the imported file as written: "@SyliusAdminBundle/Resources/config/routing/ajax.yml".
	Resource string
	Prefix   string
}

// What the code stores and who touches it. A shared table is the coupling an
// import graph never shows: two modules that never import each other but both
// write `orders` cannot be split apart, and that is what sinks most
// extractions. An entity is read from how an ORM or a mapping file declares
// one; an access from a repository's type, an ORM call, or SQL in a string.

// An Entity as declared: a class an ORM maps, or a table a mapping file or
// schema names.
type Entity struct {
	// Class is the declaring type's simple name, or its qualified name when a
	// mapping file names it; empty for a schema's model.
	Class     string
	Name      string
	Table     string
	Store     string
	Framework string
	File      string
	Line      int
}

// An Access is code using an entity or a table.
type Access struct {
	// Class is the type the access is written in, Member the function, when
	// the reader can tell; Line finds the function otherwise.
	Class  string
	Target string
	Access string
	Via    string
	File   string
	Line   int
}

const (
	AccessRead      = "read"
	AccessWrite     = "write"
	AccessReadWrite = "read_write"
)

// Where an implementation is bound to what it implements. The interface lives
// in one place, the implementation in another, and a third wires them, so the
// import graph shows the first two as unrelated and the third as touching
// both for no visible reason. A binding is read where a container is told it
// outright; a type a container finds by scanning is added when linking, from
// its annotations and what it implements.

type Binding struct {
	// Interface is what is asked for: a type's name, or a bean or service id
	// when a container names it by string.
	Interface      string
	Implementation string
	Mechanism      string
	File           string
	Line           int
	// DeclLine is a @Bean method's declaration, for reading what it returns.
	DeclLine int
}

// Pairs reads bindings from a pattern whose first two groups are the
// interface and the implementation, or the other way round.
func Pairs(s *Source, re *regexp.Regexp, mechanism string, implFirst bool) []Binding {
	var out []Binding
	for _, m := range re.FindAllStringSubmatchIndex(s.Code, -1) {
		a, b := s.Code[m[2]:m[3]], s.Code[m[4]:m[5]]
		if implFirst {
			a, b = b, a
		}
		out = append(out, Bind(s, a, b, mechanism, m[0]))
	}
	return out
}

// Bind is a binding read at an offset.
func Bind(s *Source, iface, impl, mechanism string, offset int) Binding {
	return Binding{Interface: strings.TrimSpace(iface), Implementation: strings.TrimSpace(impl), Mechanism: mechanism, File: s.Path, Line: s.Line(offset)}
}

// Where the code goes out: an HTTP call, a message sent, a database opened,
// a mail sent, a file put in a bucket. Read from how each client library is
// spelled, gated on the receiver being that client (its declared type, or
// a name no other kind of object goes by), so `map.get(key)` is never an
// HTTP GET. The address is rendered from what the call was given --
// literals, constants, configuration keys, environment variables -- and
// resolved against the configuration files when the snapshot is linked.

type OutCall struct {
	Kind, Client, Method string
	// Addr is the address as rendered: literal text with `{config:KEY}`,
	// `{env:NAME}`, `{prop:Type:member}` and `{var:name}` for the parts that
	// are not literal.
	Addr string
	// Target is a name the client declares itself by: Feign's name, a
	// MicroProfile configKey, a named HttpClient, a gRPC service, a topic.
	Target string
	// Base is a declared client's base address, rendered like Addr.
	Base       string
	Resilience string
	// Declared marks a client interface method (Feign, Retrofit, Refit,
	// HttpExchange, MicroProfile) rather than a call in code.
	Declared bool
	// Class is the client interface a declared method is on; Recv the
	// variable an imperative call was made on, for finding its base.
	Class, Recv string
	Line        int
}

// A ClientBase is a base address bound to a client variable, a bean, a
// typed or named HttpClient, or a Refit interface.
type ClientBase struct {
	Name, Addr, Resilience string
	Line                   int
}

// What a route reaches is read one call at a time: `orders.place(cart)` in a
// handler is a call into whatever type `orders` was declared as. A file's
// symbols are what it takes to follow such a call without a compiler: the
// declared type of every field, property, parameter and constructed
// variable, the string constants, which fields hold configuration, and every
// call site with the line it is on. Scope is the file, not the block: two
// variables of one name in one file with different types are rare enough to
// accept the first.

type CallSite struct {
	Recv string // as written, `this.`/`self.`/`$this->` dropped: "orders", "OrderService", "h.svc", ""
	Name string
	Line int32
	// At is where the call is written, Open its opening parenthesis.
	At, Open int32
}

type Symbols struct {
	// Vars is a name's declared type, as written (generics kept).
	Vars map[string]string
	// Consts is a string constant's value.
	Consts map[string]string
	// Keys is the configuration key a field or parameter is bound to:
	// `@Value("${partner.url}") String partnerUrl`.
	Keys map[string]string
	// Prefixes are the configuration prefixes bound to the types in the
	// file, by type name.
	Prefixes map[string]string
	// Addrs is what a field or variable was assigned when that is an
	// address: `this.baseUrl = process.env.PARTNER_URL`.
	Addrs map[string]string
	Calls []CallSite
	// AuthRules are path patterns with the access a security configuration
	// gives them, AuthRanges the line ranges a middleware group covers.
	AuthRules  []AuthRule
	AuthRanges []AuthRange
	// AppPaths are the paths an application is mounted under.
	AppPaths []string

	intern map[string]string
}

// NewSymbols is an empty symbol table.
func NewSymbols() *Symbols {
	return &Symbols{Vars: map[string]string{}, Consts: map[string]string{}, Keys: map[string]string{}, Prefixes: map[string]string{}, Addrs: map[string]string{}, intern: map[string]string{}}
}

// In is a string the table keeps: one copy of each, never a slice of the
// file it was read from.
func (sy *Symbols) In(v string) string {
	if x, ok := sy.intern[v]; ok {
		return x
	}
	c := strings.Clone(v)
	sy.intern[c] = c
	return c
}

// SetVar declares a name's type, unless it was declared already.
func (sy *Symbols) SetVar(name, typ string) {
	name, typ = strings.TrimPrefix(strings.TrimSpace(name), "$"), strings.TrimSpace(typ)
	if name == "" || typ == "" {
		return
	}
	if _, ok := sy.Vars[name]; !ok {
		sy.Vars[sy.In(name)] = sy.In(strings.Join(strings.Fields(typ), ""))
	}
}

// SetConst records a constant's value.
func (sy *Symbols) SetConst(name, value string) { sy.Consts[sy.In(name)] = sy.In(value) }

type AuthRule struct {
	Pattern, Method, Rule string
	Line                  int
}

type AuthRange struct {
	From, To int
	Rule     string
}
