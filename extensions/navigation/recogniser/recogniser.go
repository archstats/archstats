// Package recogniser is the contract between the navigation extension and
// the languages and frameworks it reads through plug-ins (ADR 0025).
// navigation itself knows no language: it reads a file with the lexical
// rules its language registered, asks that language's recognisers what the
// file says -- routes, entities, accesses, bindings, outbound calls, the
// symbols a call is followed through -- and links what they found.
//
// The package also holds what every recogniser reads with: a file with its
// comments blanked and its strings kept, annotations and declarations, the
// argument readers, and the address renderer. A plug-in imports this
// package, never navigation.
package recogniser

import (
	"regexp"
)

// A Plugin is what one language family brings: its languages and the
// recognisers that read them. The engine hands navigation one per family.
type Plugin struct {
	Name string
	// Languages are the languages the family owns, by file extension.
	Languages []*Language
	// Recognisers read files of the languages each names, the family's own
	// or another's: Spring's XML beans are read from an XML file.
	Recognisers []*Recogniser
}

// Kind says what a language's files are read for.
type Kind int

const (
	// Code is a program: its declarations, calls and symbols are read.
	Code Kind = iota
	// Data declares without running: a mapping file, a schema, routes in
	// YAML. Its facts count even when the file is not the codebase's own.
	Data
	// Prose is documentation, read for what it says and names.
	Prose
)

// A Language is how a family of files is written, as far as a lexical
// reader needs to know.
type Language struct {
	// Name identifies the language in facts: "java", "csharp", "js".
	Name       string
	Extensions []string
	Kind       Kind
	Syntax     Syntax

	// recognisers are the language's recognisers, attached by Bind.
	recognisers []*Recogniser
}

// Recognisers are the language's recognisers, in the order their plug-ins
// were given.
func (l *Language) Recognisers() []*Recogniser { return l.recognisers }

// Bind attaches a recogniser to the language. navigation binds each
// recogniser to every language it names, once, when the extension is built.
func (l *Language) Bind(r *Recogniser) { l.recognisers = append(l.recognisers, r) }

// A Recogniser reads what one language, or one framework family, spells.
// Every field is optional; navigation calls each where its own reading asks
// the question, in the order the plug-ins were given.
type Recogniser struct {
	// Languages are the names of the languages read.
	Languages []string

	// Entry points.

	// ClassMark reads what a type's mark says about the members below it.
	ClassMark func(m Mark, c *ClassContext)
	// ClassEntries are the entry points a type is by its marks alone.
	ClassEntries func(s *Source, d Decl) []Entry
	// MemberEntries are the entry points an annotated member is. e carries
	// what every entry of the member shares: its lines, its class, and what
	// its marks say about access and documentation.
	MemberEntries func(s *Source, d Decl, class ClassContext, e Entry) []Entry
	// CallEntries are routes declared by calling something.
	CallEntries func(s *Source) []Entry
	// MainEntries are the programs a file starts.
	MainEntries func(s *Source) []Entry
	// FileEntries are routes a framework reads from where a file is.
	FileEntries func(path string) []Entry
	// Routes reads a routing file's routes and the routing files it mounts.
	Routes func(s *Source) ([]Entry, []RouteImport)
	// RoutingFile is whether a path is a routing file, whose routes are
	// mounted under the prefix of the routing files that import it.
	RoutingFile func(path string) bool
	// ImportTargets are the routing files an import's resource names.
	ImportTargets func(from, resource string, routing []string) []string
	// RouteConfigs are the frameworks that route by file location, each
	// with the config files that must be in the workspace for its candidate
	// routes to count.
	RouteConfigs map[string][]string
	// Convention routes measured functions by name and signature alone.
	Convention *Convention
	// MethodsByName reads a route's methods from its handler's names where
	// the framework leaves them there and the route says none: a Django
	// class view answers GET with get(), a Next.js route file with its
	// exported GET, an ASP.NET Web API action named GetOrders answers GET.
	// Nil, or nothing returned, leaves the route taking any method.
	MethodsByName func(h Handler) []MemberMethod
	// MemberMethods are the methods a member's own marks restrict it to,
	// comma-joined, where its route says none: Django's @require_POST, or
	// Django REST framework's @api_view, which is GET when it lists none.
	// "" says nothing.
	MemberMethods func(d Decl) string
	// AuthMarks name the marks that say who may call a route, beyond those
	// whose names say so; DocMarks the marks that describe a route for its
	// contract.
	AuthMarks []string
	DocMarks  []string

	// Data.

	Entities func(s *Source) []Entity
	Accesses func(s *Source) []Access
	// QueryVia is what SQL written in a file's strings is when it may name
	// entities rather than tables: "sql|jpql"; "" when it is just SQL.
	QueryVia func(s *Source) string
	// ModelBase says whether a supertype makes a class a mapped model, by
	// the framework that maps it. decided without a framework means the
	// supertype is not one and its own supertypes are not looked into.
	ModelBase func(supertype string, imports func() []string) (framework string, decided bool)

	// Wiring.

	Bindings func(s *Source) []Binding
	// Scanned are the marks under which a container finds a type by
	// scanning and binds it to what it implements.
	Scanned []string

	// Symbols.

	// Declared reads the declared type of names: fields, parameters,
	// properties.
	Declared func(s *Source, sy *Symbols)
	// Constructed rewrites a constructed type as written to the type it
	// names: Go's NewOrders is Orders.
	Constructed func(typ string) string
	// Modules reads names bound to what a module constructs.
	Modules   func(s *Source, sy *Symbols)
	Constants func(s *Source, sy *Symbols)
	// ConfigKeys reads configuration bound to fields and the prefixes bound
	// to types.
	ConfigKeys func(s *Source, sy *Symbols)
	// Security reads the access a security configuration gives paths.
	Security func(s *Source, sy *Symbols)
	// ClassAuth is what each class says about access to every route on it.
	ClassAuth func(s *Source) map[string]string

	// Outbound calls.

	// ClientBases are the base addresses a file binds to clients.
	ClientBases func(s *Source, sy *Symbols) []ClientBase
	// DeclaredClients are the methods of client interfaces, each a call.
	DeclaredClients func(s *Source) []OutCall
	// Calls reads the outbound calls spelled only this language's way.
	Calls func(s *Source, sy *Symbols, add func(c OutCall, at int))
	// ClientName is the library an HTTP client receiver is, from its type
	// (lower-cased) or name, before the libraries that name themselves are
	// looked for; "" when it does not say.
	ClientName func(s *Source, typ, recv string) string
	// BareClient is whether a variable typed just Client is an HTTP client.
	BareClient func(s *Source) bool
	// ClientModules are module names that are HTTP clients in this language
	// alone; NotClients are receivers that are never one.
	ClientModules []string
	NotClients    []string
	// Resilience maps the marks that make a call resilient to what they
	// add: "timeout", "retry", "circuit_breaker".
	Resilience map[string]string
	// OutRules and CallRules read outbound calls by table.
	OutRules  []OutRule
	CallRules []CallRule

	// Configuration.

	// ConfigFormat is how a file of this language holds runtime
	// configuration, or "".
	ConfigFormat func(path string) string
	// ReadConfig reads a file in a format ConfigFormat named.
	ReadConfig func(path, format, text string, add func(key, value string, line int, env string))
}

// A Convention routes a measured function by its name and signature alone:
// ASP.NET MVC's /{controller}/{action}.
type Convention struct {
	// Framework names the routes it makes; Completes is the framework whose
	// routes on the same function it completes when they have no path.
	Framework, Completes string
	// Route is the function's conventional path, if it has one.
	Route func(function, signature string) (string, bool)
}

// A Handler is what MethodsByName reads: the route's framework, the
// handler's type as declared (its signature; "" for a file), the names of
// the members that may answer (the type's or the file's functions, or the
// one function routed), and whether the workspace declares a type of a name,
// so a base the project wrote itself is not taken for the framework's.
type Handler struct {
	Framework, Declaration string
	Members                []string
	Declared               func(typeName string) bool
}

// A MemberMethod is a method a handler answers and the member that answers
// it; an empty member is the handler itself (a Django ListView's GET, from
// its base).
type MemberMethod struct{ Method, Member string }

// An OutRule reads one spelling of an outbound call by pattern.
type OutRule struct {
	// Langs narrows the rule to some of its recogniser's languages; nil
	// reads it in all of them.
	Langs  []string
	Re     *regexp.Regexp // ends at the opening ( or { of the call's arguments
	Kind   string
	Client string
	Method string // fixed
	Arg    int    // the address or topic argument; -1 for none
	Key    string // or the named key that holds it: `topic:`, `QueueUrl=`, `Topic:`
	Gate   string // a pattern the receiver's name or type must match
}

// A CallRule reads an outbound call made on a receiver:
// `kafkaTemplate.send(topic, …)`. It is matched against the call sites the
// symbols already found, so no pattern has to start at every identifier.
type CallRule struct {
	Langs   []string
	Members *regexp.Regexp // the member called
	Recv    *regexp.Regexp // the receiver as written, when the gate is not enough
	Tail    *regexp.Regexp // what must follow, from the opening parenthesis
	Kind    string
	Client  string
	Arg     int
	Key     string
	Gate    string
}

// Reads says whether the rule reads a file.
func (rl *OutRule) Reads(s *Source) bool { return in(rl.Langs, s.Lang) }

// Reads says whether the rule reads a file.
func (rl *CallRule) Reads(s *Source) bool { return in(rl.Langs, s.Lang) }

func in(langs []string, lang string) bool {
	return langs == nil || Contains(langs, lang)
}

// Clone is a copy of the language with no recognisers bound, for one
// extension to bind its own to.
func (l *Language) Clone() *Language {
	c := *l
	c.recognisers = nil
	return &c
}
