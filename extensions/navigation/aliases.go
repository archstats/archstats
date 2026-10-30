package navigation

import r "github.com/archstats/archstats/extensions/navigation/recogniser"

// The facts recognisers read, as navigation links them.
type (
	entry        = r.Entry
	entity       = r.Entity
	access       = r.Access
	binding      = r.Binding
	typeDecl     = r.TypeDecl
	routeImport  = r.RouteImport
	outCall      = r.OutCall
	clientBase   = r.ClientBase
	symbols      = r.Symbols
	callSite     = r.CallSite
	authRule     = r.AuthRule
	authRange    = r.AuthRange
	mark         = r.Mark
	decl         = r.Decl
	source       = r.Source
	classContext = r.ClassContext
)

const (
	kindHTTP     = r.KindHTTP
	kindPage     = r.KindPage
	kindMessage  = r.KindMessage
	kindEvent    = r.KindEvent
	kindSchedule = r.KindSchedule
	kindCLI      = r.KindCLI
	kindMain     = r.KindMain
	kindJob      = r.KindJob

	accessRead      = r.AccessRead
	accessWrite     = r.AccessWrite
	accessReadWrite = r.AccessReadWrite
)

var (
	rx            = r.Rx
	balanced      = r.Balanced
	splitArgs     = r.SplitArgs
	unquote       = r.Unquote
	lastName      = r.LastName
	joinRoute     = r.JoinRoute
	joinAddr      = r.JoinAddr
	genericless   = r.Genericless
	lastSegment   = r.LastSegment
	normalRecv    = r.NormalRecv
	isIdentifier  = r.IsIdentifier
	isQualified   = r.IsQualified
	isWordByte    = r.IsWordByte
	positional    = r.Positional
	objectKey     = r.ObjectKey
	looksLikeAddr = r.LooksLikeAddr
	methodLiteral = r.MethodLiteral
	resilienceIn  = r.ResilienceIn
	itoa          = r.Itoa
	contains      = r.Contains
	authish       = r.Authish
	notCalls      = r.NotCalls
	orDefault     = r.OrDefault
)
