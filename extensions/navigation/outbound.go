package navigation

import (
	"regexp"
	"regexp/syntax"
	"strings"
)

// Where the code goes out: an HTTP call, a message sent, a database opened,
// a mail sent, a file put in a bucket. Read from how each client library is
// spelled, gated on the receiver being that client (its declared type, or
// a name no other kind of object goes by), so `map.get(key)` is never an
// HTTP GET. The address is rendered from what the call was given --
// literals, constants, configuration keys, environment variables -- and
// resolved against the configuration files when the snapshot is linked.

type outCall struct {
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

// A clientBase is a base address bound to a client variable, a bean, a
// typed or named HttpClient, or a Refit interface.
type clientBase struct {
	Name, Addr, Resilience string
	Line                   int
}

var (
	jsLike     = map[string]bool{langJS: true}
	jvm        = map[string]bool{langJava: true, langKotlin: true, langScala: true}
	httpTypes  = regexp.MustCompile(`^(?:RestTemplate|TestRestTemplate|WebClient|RestClient|HttpClient|OkHttpClient|CloseableHttpClient|HttpClients|WebTarget|HttpService|AxiosInstance|AxiosStatic|Session|requests\.Session|httpx\.Client|httpx\.AsyncClient|aiohttp\.ClientSession|ClientSession|http\.Client|resty\.Client|ClientInterface|HttpClientInterface|IRestClient|RestSharp\.RestClient|IFlurlClient|FlurlClient|HttpRequestFactory|Client)$`)
	httpNames  = regexp.MustCompile(`(?i)(^|_)(http|rest|web|api)|(http|rest|web|api)(client|template|service)?$|^_?(http|client|session|httpclient|resttemplate|webclient)$`)
	httpModule = map[string]bool{"axios": true, "requests": true, "httpx": true, "got": true, "ky": true, "superagent": true, "Http": true, "Unirest": true, "unirest": true, "needle": true, "ofetch": true}
	verbMember = regexp.MustCompile(`(?i)^(get|post|put|patch|delete|del|head|options)(?:for(?:object|entity|location))?(?:as(?:json|xml))?(?:async)?$|^(get)(?:string|stream|bytearray|fromjson)(?:async)?$|^(delete)fromjson(?:async)?$`)
	requestish = map[string]bool{"request": true, "requestasync": true, "exchange": true, "send": true, "sendasync": true, "execute": true, "fetch": true}

	webClientChain = regexp.MustCompile(`([A-Za-z_$][\w$.]*)\s*\.\s*(get|post|put|patch|delete|head|options|method)\s*\(\s*(?:HttpMethod\.(\w+))?\s*\)\s*\.\s*uri\s*\(`)
	javaHTTPReq    = regexp.MustCompile(`HttpRequest\s*\.\s*newBuilder\s*\(`)
	okHTTPReq      = regexp.MustCompile(`\bRequest\s*\.\s*Builder\s*\(\s*\)`)
	apacheReq      = regexp.MustCompile(`\bnew\s+Http(Get|Post|Put|Delete|Patch|Head|Options)\s*\(`)
	jaxrsTarget    = regexp.MustCompile(`\.\s*target\s*\(`)
	csRequestMsg   = regexp.MustCompile(`\bnew\s+HttpRequestMessage\s*\(\s*(?:HttpMethod\.(\w+)|new\s+HttpMethod\s*\(\s*"(\w+)"\s*\))\s*,`)
	restSharpReq   = regexp.MustCompile(`\bnew\s+RestRequest\s*\(`)
	jsFetch        = regexp.MustCompile(`(?:^|[^\w$.])(fetch|\$fetch|useFetch|useLazyFetch|ofetch)\s*(?:<[^<>()]*>)?\s*\(`)
	jsAxiosConfig  = regexp.MustCompile(`\baxios\s*\(\s*\{`)
	laravelHTTP    = regexp.MustCompile(`\bHttp::`)
	restyChain     = regexp.MustCompile(`\.R\s*\(\s*\)((?:\s*\.\s*\w+\s*\([^()]*(?:\([^()]*\)[^()]*)*\))*?)\s*\.\s*(Get|Post|Put|Patch|Delete|Head|Options)\s*\(`)
	goHTTPCall     = regexp.MustCompile(`\bhttp\s*\.\s*(Get|Post|PostForm|Head)\s*\(`)
	goNewRequest   = regexp.MustCompile(`\bhttp\s*\.\s*NewRequest(?:WithContext)?\s*\(`)
	phpCurl        = regexp.MustCompile(`\bcurl_init\s*\(|CURLOPT_URL\s*,`)
	pyURLOpen      = regexp.MustCompile(`\b(?:urlopen|urllib\.request\.Request|urllib2\.Request|Request)\s*\(`)
	unirestCall    = regexp.MustCompile(`\b[Uu]nirest\s*\.\s*(get|post|put|patch|delete|head|options)\s*\(`)
	celeryCall     = regexp.MustCompile(`\b([a-z_]\w*)\s*\.\s*(delay|apply_async)\s*\(`)
	resilienceMark = map[string]string{"Retry": "retry", "Retryable": "retry", "CircuitBreaker": "circuit_breaker", "TimeLimiter": "timeout", "Timeout": "timeout", "Bulkhead": "retry", "Fallback": "circuit_breaker", "RateLimiter": "retry", "retry": "retry", "backoff": "retry", "on_exception": "retry", "circuit": "circuit_breaker"}
	timeoutArg     = regexp.MustCompile(`(?i)\b(timeout|connect_?timeout|read_?timeout|request_?timeout|callTimeout|responseTimeout|Timeout)\s*[:=(]|->timeout\s*\(|\.timeout\s*\(|\.Timeout\s*=|['"]timeout['"]\s*=>`)
	retryArg       = regexp.MustCompile(`(?i)->retry\s*\(|\.retry(?:When|Backoff)?\s*\(|max_retries|retries\s*[:=]|AddPolicyHandler|AddTransientHttpErrorPolicy|AddStandardResilienceHandler|AddResilienceHandler|Retry\.(?:backoff|fixedDelay|max)`)
	circuitArg     = regexp.MustCompile(`(?i)CircuitBreaker|circuitBreaker|AddStandardResilienceHandler`)
)

// A rule reads one spelling of an outbound call.
type outRule struct {
	langs  map[string]bool
	re     *regexp.Regexp // ends at the opening ( or { of the call's arguments
	kind   string
	client string
	method string // fixed; "member" reads the method from the member called; "arg0" from the first argument
	arg    int    // the address or topic argument; -1 for none
	key    string // or the named key that holds it: `topic:`, `QueueUrl=`, `Topic:`
	gate   string // "http": the receiver must be an HTTP client; else a pattern its name or type must match
}

var (
	clientBaseHint = regexp.MustCompile(`axios|WebClient|RestClient|@Bean|BaseAddress|AddHttpClient|AddRefitClient|RestService|httpx|GuzzleHttp|base_uri|createForBaseUri|resty`)
)

func lang(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

var allLangs = lang(langJava, langKotlin, langScala, langCSharp, langGo, langJS, langPython, langPHP, langDart, langSwift)

var outRules = []outRule{
	// Messaging.
	{lang(langCSharp), regexp.MustCompile(`\bCreateSender\s*\(`), "message", "azure-service-bus", "", 0, "", ""},
	{lang(langCSharp), regexp.MustCompile(`\bBasicPublish(?:Async)?\s*\(`), "message", "rabbitmq", "", 0, "", ""},
	{jsLike, regexp.MustCompile(`\b([\w$.]+)\s*\.\s*(sendToQueue)\s*\(`), "message", "amqp", "", 0, "", ""},
	{jsLike, regexp.MustCompile(`\bnew\s+SendMessage(?:Batch)?Command\s*\(\s*\{`), "message", "sqs", "", -1, "QueueUrl", ""},
	{jsLike, regexp.MustCompile(`\bnew\s+PublishCommand\s*\(\s*\{`), "message", "sns", "", -1, "TopicArn", ""},
	{jsLike, regexp.MustCompile(`\b([\w$.]*[Qq]ueue)\s*\.\s*(add)\s*\(`), "message", "bull", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\bbasic_publish\s*\(`), "message", "amqp", "", 1, "routing_key", ""},
	{lang(langPython), regexp.MustCompile(`\.\s*send_message\s*\(`), "message", "sqs", "", -1, "QueueUrl", ""},
	{lang(langPython), regexp.MustCompile(`\.\s*publish\s*\(\s*TopicArn`), "message", "sns", "", -1, "TopicArn", ""},
	{lang(langGo), regexp.MustCompile(`\bsarama\s*\.\s*ProducerMessage\s*\{`), "message", "kafka", "", -1, "Topic", ""},
	{lang(langGo), regexp.MustCompile(`\bkafka\s*\.\s*(?:Writer|Message)\s*\{`), "message", "kafka", "", -1, "Topic", ""},
	{lang(langPHP), regexp.MustCompile(`\$[\w>-]*[bB]us\s*->\s*dispatch\s*\(\s*new\s+\\?([\w\\]+)`), "message", "symfony-messenger", "", -1, "", ""},
	{lang(langPHP), regexp.MustCompile(`\bbasic_publish\s*\(`), "message", "amqp", "", 2, "", ""},

	// Databases, caches, search, mail, LDAP, storage, file transfer.
	{lang(langJava, langKotlin), regexp.MustCompile(`\bDriverManager\s*\.\s*getConnection\s*\(`), "sql", "jdbc", "", 0, "", ""},
	{lang(langCSharp), regexp.MustCompile(`\bnew\s+(?:SqlConnection|NpgsqlConnection|MySqlConnection|OracleConnection|SqliteConnection)\s*\(`), "sql", "ado.net", "", 0, "", ""},
	{lang(langCSharp), regexp.MustCompile(`\.\s*Use(?:SqlServer|Npgsql|MySql|MySQL|Oracle|Sqlite|Cosmos)\s*\(`), "sql", "ef-core", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\b(?:create_engine|create_async_engine|psycopg2\.connect|psycopg\.connect|pymysql\.connect|asyncpg\.connect|asyncpg\.create_pool)\s*\(`), "sql", "python-db", "", 0, "dsn", ""},
	{lang(langGo), regexp.MustCompile(`\bsql\s*\.\s*Open\s*\(`), "sql", "database/sql", "", 1, "", ""},
	{lang(langGo), regexp.MustCompile(`\b(?:postgres|mysql|sqlserver)\s*\.\s*Open\s*\(`), "sql", "gorm", "", 0, "", ""},
	{lang(langGo), regexp.MustCompile(`\bpgxpool\s*\.\s*(?:New|Connect)\s*\(`), "sql", "pgx", "", 1, "", ""},
	{jsLike, regexp.MustCompile(`\bnew\s+(?:Pool|pg\.Pool|pg\.Client)\s*\(\s*\{`), "sql", "pg", "", -1, "connectionString", ""},
	{jsLike, regexp.MustCompile(`\bmysql2?\s*\.\s*create(?:Connection|Pool)\s*\(\s*\{`), "sql", "mysql", "", -1, "host", ""},
	{lang(langJava, langKotlin), regexp.MustCompile(`\b(?:MongoClients\s*\.\s*create|new\s+MongoClient)\s*\(`), "nosql", "mongodb", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\b(?:MongoClient|AsyncIOMotorClient|motor\.motor_asyncio\.AsyncIOMotorClient|pymongo\.MongoClient)\s*\(`), "nosql", "mongodb", "", 0, "host", ""},
	{jsLike, regexp.MustCompile(`\b(?:mongoose\s*\.\s*connect|mongoose\s*\.\s*createConnection|new\s+MongoClient|MongoClient\s*\.\s*connect)\s*\(`), "nosql", "mongodb", "", 0, "", ""},
	{lang(langGo), regexp.MustCompile(`\.\s*ApplyURI\s*\(`), "nosql", "mongodb", "", 0, "", ""},
	{lang(langCSharp), regexp.MustCompile(`\bnew\s+MongoClient\s*\(`), "nosql", "mongodb", "", 0, "", ""},
	{lang(langJava, langKotlin), regexp.MustCompile(`\bnew\s+(?:Jedis|JedisPool|JedisPooled)\s*\(`), "cache", "jedis", "", 0, "", ""},
	{lang(langJava, langKotlin), regexp.MustCompile(`\bRedisClient\s*\.\s*create\s*\(`), "cache", "lettuce", "", 0, "", ""},
	{lang(langCSharp), regexp.MustCompile(`\bConnectionMultiplexer\s*\.\s*Connect(?:Async)?\s*\(`), "cache", "stackexchange.redis", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\bredis\s*\.\s*(?:Redis|StrictRedis|from_url|Redis\s*\.\s*from_url|asyncio\s*\.\s*Redis)\s*\(`), "cache", "redis-py", "", 0, "host", ""},
	{jsLike, regexp.MustCompile(`\b(?:createClient|redis\s*\.\s*createClient)\s*\(\s*\{`), "cache", "redis", "", -1, "url", ""},
	{jsLike, regexp.MustCompile(`\bnew\s+(?:Redis|IORedis)\s*\(`), "cache", "ioredis", "", 0, "", ""},
	{lang(langGo), regexp.MustCompile(`\bredis\s*\.\s*NewClient\s*\(\s*&\s*redis\s*\.\s*Options\s*\{`), "cache", "go-redis", "", -1, "Addr", ""},
	{lang(langPython), regexp.MustCompile(`\bElasticsearch\s*\(`), "search", "elasticsearch", "", 0, "hosts", ""},
	{jsLike, regexp.MustCompile(`\bnew\s+(?:elasticsearch\s*\.\s*)?Client\s*\(\s*\{\s*node`), "search", "elasticsearch", "", -1, "node", ""},
	{lang(langJava, langKotlin), regexp.MustCompile(`\bRestClient\s*\.\s*builder\s*\(\s*(?:new\s+)?HttpHost`), "search", "elasticsearch", "", -1, "", ""},
	{lang(langCSharp), regexp.MustCompile(`\bnew\s+SmtpClient\s*\(`), "mail", "smtp", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\bsmtplib\s*\.\s*SMTP(?:_SSL)?\s*\(`), "mail", "smtp", "", 0, "host", ""},
	{jsLike, regexp.MustCompile(`\bnodemailer\s*\.\s*createTransport\s*\(\s*\{`), "mail", "nodemailer", "", -1, "host", ""},
	{lang(langPHP), regexp.MustCompile(`\bMail::(?:to|send|queue|raw)\s*\(`), "mail", "laravel-mail", "", -1, "", ""},
	{lang(langPython), regexp.MustCompile(`\bldap3\s*\.\s*Server\s*\(|\bServer\s*\(\s*['"]ldaps?://`), "ldap", "ldap3", "", 0, "", ""},
	{lang(langPHP), regexp.MustCompile(`\bldap_connect\s*\(`), "ldap", "php-ldap", "", 0, "", ""},
	{lang(langCSharp), regexp.MustCompile(`\bnew\s+(?:LdapConnection|DirectoryEntry)\s*\(`), "ldap", "directory-services", "", 0, "", ""},
	{lang(langJava, langKotlin), regexp.MustCompile(`\bnew\s+(?:InitialDirContext|InitialLdapContext)\s*\(`), "ldap", "jndi", "", -1, "", ""},
	{lang(langPython), regexp.MustCompile(`\bboto3\s*\.\s*(?:client|resource)\s*\(\s*['"]s3['"]`), "storage", "s3", "", -1, "", ""},
	{lang(langJava, langKotlin, langCSharp, langGo, langJS), regexp.MustCompile(`\b(?:S3Client\s*\.\s*(?:builder|create)|new\s+S3Client|AmazonS3ClientBuilder|new\s+AmazonS3Client|s3\s*\.\s*New(?:FromConfig)?|new\s+BlobServiceClient|new\s+BlobContainerClient|BlobServiceClient\s*\.\s*fromConnectionString|StorageOptions\s*\.\s*getDefaultService|storage\s*\.\s*NewClient|new\s+Storage)\s*\(`), "storage", "object-storage", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\b(?:BlobServiceClient(?:\s*\.\s*from_connection_string)?|storage\s*\.\s*Client|Minio)\s*\(`), "storage", "object-storage", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\bftplib\s*\.\s*FTP(?:_TLS)?\s*\(|\bFTP(?:_TLS)?\s*\(`), "file_transfer", "ftp", "", 0, "host", ""},
	{lang(langJava, langKotlin, langCSharp), regexp.MustCompile(`\bnew\s+(?:FTPClient|FTPSClient|SftpClient|FtpClient|AsyncFtpClient)\s*\(`), "file_transfer", "ftp", "", 0, "", ""},
	{jsLike, regexp.MustCompile(`\b([\w$.]*(?:sftp|ftp)\w*)\s*\.\s*(connect|access)\s*\(\s*\{`), "file_transfer", "sftp", "", -1, "host", ""},

	// gRPC, GraphQL, SOAP.
	{lang(langJava, langKotlin), regexp.MustCompile(`\b(\w+)Grpc\s*\.\s*new(?:Blocking|Future)?(?:V2)?Stub\s*\(`), "grpc", "grpc-java", "", -1, "", ""},
	{lang(langJava, langKotlin), regexp.MustCompile(`\bManagedChannelBuilder\s*\.\s*for(?:Target|Address)\s*\(`), "grpc", "grpc-java", "", 0, "", ""},
	{lang(langGo), regexp.MustCompile(`\b\w+\s*\.\s*New(\w+)Client\s*\(\s*\w+\s*\)`), "grpc", "grpc-go", "", -1, "", ""},
	{lang(langGo), regexp.MustCompile(`\bgrpc\s*\.\s*(?:Dial|DialContext|NewClient)\s*\(`), "grpc", "grpc-go", "", 0, "", ""},
	{lang(langCSharp), regexp.MustCompile(`\bGrpcChannel\s*\.\s*ForAddress\s*\(`), "grpc", "grpc-dotnet", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\bgrpc\s*\.\s*(?:aio\s*\.\s*)?(?:insecure|secure)_channel\s*\(`), "grpc", "grpcio", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\b\w+_pb2_grpc\s*\.\s*(\w+)Stub\s*\(`), "grpc", "grpcio", "", -1, "", ""},
	{jsLike, regexp.MustCompile(`\bnew\s+ApolloClient\s*\(\s*\{|\bnew\s+HttpLink\s*\(\s*\{|\bcreateHttpLink\s*\(\s*\{`), "graphql", "apollo", "", -1, "uri", ""},
	{jsLike, regexp.MustCompile(`\bnew\s+GraphQLClient\s*\(`), "graphql", "graphql-request", "", 0, "", ""},
	{lang(langCSharp), regexp.MustCompile(`\bnew\s+GraphQLHttpClient\s*\(`), "graphql", "graphql-client", "", 0, "", ""},
	{lang(langPython), regexp.MustCompile(`\bzeep\s*\.\s*(?:Client|AsyncClient)\s*\(|\bClient\s*\(\s*(?:wsdl\s*=\s*)?['"][^'"]+\?wsdl`), "soap", "zeep", "", 0, "wsdl", ""},
	{lang(langPHP), regexp.MustCompile(`\bnew\s+\\?SoapClient\s*\(`), "soap", "php-soap", "", 0, "", ""},
	{lang(langJava, langKotlin), regexp.MustCompile(`\bENDPOINT_ADDRESS_PROPERTY\s*,`), "soap", "jax-ws", "", 0, "", ""},
}

// A callRule reads an outbound call made on a receiver: `kafkaTemplate.send(topic, …)`.
// It is matched against the call sites the symbols already found, so no
// pattern has to start at every identifier in a file.
type callRule struct {
	langs   map[string]bool
	members *regexp.Regexp // the member called
	recv    *regexp.Regexp // the receiver as written, when the gate is not enough
	tail    *regexp.Regexp // what must follow, from the opening parenthesis
	kind    string
	client  string
	arg     int
	key     string
	gate    string
}

var callRules = []callRule{
	{langs: lang(langJava, langKotlin, langScala), members: regexp.MustCompile(`^(?:send|sendDefault)$`), kind: "message", client: "kafka", arg: 0, gate: `(?i)kafka`},
	{langs: lang(langJava, langKotlin, langScala), members: regexp.MustCompile(`^(?:convertAndSend|send)$`), kind: "message", client: "amqp", arg: 0, gate: `(?i)rabbit|amqp`},
	{langs: lang(langJava, langKotlin, langScala), members: regexp.MustCompile(`^(?:convertAndSend|send)$`), kind: "message", client: "jms", arg: 0, gate: `(?i)jms`},
	{langs: lang(langJava, langKotlin, langScala), members: regexp.MustCompile(`^(?:send)$`), kind: "message", client: "spring-cloud-stream", arg: 0, gate: `(?i)streambridge`},
	{langs: lang(langCSharp), members: regexp.MustCompile(`^(?:ProduceAsync|Produce)$`), kind: "message", client: "kafka", arg: 0, gate: `(?i)produc`},
	{langs: jsLike, members: regexp.MustCompile(`^(?:send|sendBatch)$`), tail: regexp.MustCompile(`^\(\s*\{`), kind: "message", client: "kafka", arg: -1, key: "topic", gate: `(?i)produc|kafka`},
	{langs: jsLike, members: regexp.MustCompile(`^(?:publish)$`), kind: "message", client: "amqp", arg: 0, gate: `(?i)^(ch|channel|amqp\w*|rabbit\w*|confirmChannel)$`},
	{langs: jsLike, members: regexp.MustCompile(`^(?:emit|send)$`), kind: "message", client: "nestjs-microservices", arg: 0, gate: `(?i)ClientProxy|^(client|\w+Client|\w+Service)$`},
	{langs: lang(langPython), members: regexp.MustCompile(`^(?:send|produce|send_and_wait)$`), kind: "message", client: "kafka", arg: 0, key: "topic", gate: `(?i)produc|kafka`},
	{langs: lang(langGo), members: regexp.MustCompile(`^(?:Publish|PublishWithContext)$`), kind: "message", client: "amqp", arg: 1, gate: `(?i)^(ch|channel|amqp\w*|rabbit\w*)$`},
	{langs: lang(langGo), members: regexp.MustCompile(`^(?:Publish|Request)$`), kind: "message", client: "nats", arg: 0, gate: `(?i)^(nc|js|nats\w*|conn)$`},
	{langs: lang(langJava, langKotlin), members: regexp.MustCompile(`^(?:send)$`), kind: "mail", client: "javamail", arg: -1, gate: `(?i)mailsender|javamailsender|^mail`},
	{langs: lang(langPython), members: regexp.MustCompile(`^(?:connect)$`), recv: regexp.MustCompile(`^(?:[\w.]*(?:ssh|sftp|client)\w*)$`), kind: "file_transfer", client: "paramiko", arg: 0, key: "hostname", gate: `(?i)ssh|sftp|paramiko`},
	{langs: lang(langJava, langKotlin), members: regexp.MustCompile(`^(?:getSession)$`), kind: "file_transfer", client: "jsch", arg: 1, gate: `(?i)jsch`},
	{langs: lang(langJava, langKotlin), members: regexp.MustCompile(`^(?:marshalSendAndReceive|sendSourceAndReceive)$`), kind: "soap", client: "spring-ws", arg: 0, gate: `(?i)webservicetemplate|^ws|soap`},
	{langs: lang(langJava, langKotlin), members: regexp.MustCompile(`^setAddress$`), kind: "soap", client: "jax-ws", arg: 0, gate: `(?i)factory|proxy|jaxws`},
	{langs: lang(langCSharp), members: regexp.MustCompile(`^(?:Publish|Send)$`), tail: regexp.MustCompile(`^\(\s*new\s+([A-Z]\w*)`), kind: "message", client: "masstransit", arg: -1, gate: `(?i)bus|publish|endpoint|sendendpoint`},
}

// outbound reads every outbound call in a file.
func (s *source) outbound(sy *symbols) ([]outCall, []clientBase) {
	if s.lang == langUnreadable || s.lang == langMarkdown || s.lang == langXML || s.lang == langYAML || s.lang == langPrisma {
		return nil, nil
	}
	code := s.code
	var out []outCall
	seen := map[int]bool{}
	add := func(c outCall, at int) {
		if seen[at] {
			return
		}
		seen[at] = true
		c.Line = s.line(at)
		if c.Resilience == "" {
			c.Resilience = resilienceIn(s.statement(at))
		}
		out = append(out, c)
	}
	s.addresses(sy)
	bases := s.clientBases(sy)

	// Declared clients: Feign, MicroProfile, HttpExchange, Retrofit, Refit.
	out = append(out, s.declaredClients()...)

	// Imperative HTTP calls on a client variable: x.get(url), x.PostAsync(url).
	for _, cs := range sy.calls {
		if cs.Recv == "" {
			continue
		}
		recv, member := cs.Recv, cs.Name
		method := httpVerbOf(member)
		reqish := requestish[strings.ToLower(member)]
		if method == "" && !reqish {
			continue
		}
		if !s.isHTTPClient(recv, sy, bases) {
			continue
		}
		inner, _ := balanced(code, int(cs.Open))
		args := splitArgs(inner)
		if len(args) == 0 {
			continue
		}
		addr := args[0]
		if reqish || method == "" {
			if mm := methodLiteral(args[0]); mm != "" {
				method = mm
				if len(args) < 2 {
					continue
				}
				addr = args[1]
			} else if len(args) > 1 {
				if mm := methodLiteral(args[1]); mm != "" {
					method = mm
				}
			}
			if method == "" {
				method = "ANY"
			}
		}
		rendered := s.renderAddr(addr, sy)
		_, typed := sy.vars[lastSegment(recv)]
		if rendered == "" || !looksLikeAddr(rendered, addr) && !(typed && method != "") {
			continue
		}
		add(outCall{Kind: "http", Client: clientName(recv, sy, s.lang), Method: method, Addr: rendered, Recv: lastSegment(recv)}, int(cs.At))
	}

	// Calls on a receiver that sends a message, sends mail, opens a session.
	lower := ""
	for i, r := range callRules {
		if !r.langs[s.lang] {
			continue
		}
		if words := callGateWords[i]; len(words) > 0 {
			if lower == "" {
				lower = strings.ToLower(code)
			}
			if !containsAny(lower, words) {
				continue
			}
		}
		for _, cs := range sy.calls {
			if cs.Recv == "" || !r.members.MatchString(cs.Name) {
				continue
			}
			if r.recv != nil && !r.recv.MatchString(cs.Recv) || r.gate != "" && !gateMatches(r.gate, cs.Recv, sy) {
				continue
			}
			rest := code[cs.Open:min(len(code), int(cs.Open)+400)]
			var tail []int
			if r.tail != nil {
				if tail = r.tail.FindStringSubmatchIndex(rest); tail == nil {
					continue
				}
			}
			body, _ := balanced(code, int(cs.Open))
			if r.tail != nil && strings.HasSuffix(rest[:tail[1]], "{") {
				body, _ = balanced(code, int(cs.Open)+tail[1]-1)
			}
			c := outCall{Kind: r.kind, Client: r.client}
			args := splitArgs(body)
			switch {
			case r.key != "":
				v := objectKey(body, r.key)
				if v == "" && r.arg >= 0 && r.arg < len(positional(args)) {
					v = positional(args)[r.arg]
				}
				if v != "" {
					c.Addr = s.renderAddr(v, sy)
				}
			case r.arg >= 0 && r.arg < len(args):
				c.Addr = s.renderAddr(args[r.arg], sy)
			}
			if tail != nil && len(tail) > 2 && tail[2] >= 0 {
				c.Target = rest[tail[2]:tail[3]]
				if c.Addr == "" {
					c.Addr = c.Target
				}
			}
			add(c, int(cs.At))
		}
	}

	switch {
	case jvm[s.lang]:
		for _, m := range webClientChain.FindAllStringSubmatchIndex(code, -1) {
			if !strings.Contains(code, "uri(") {
				break
			}
			recv := normalRecv(code[m[2]:m[3]])
			method := strings.ToUpper(code[m[4]:m[5]])
			if m[6] >= 0 {
				method = strings.ToUpper(code[m[6]:m[7]])
			}
			inner, _ := balanced(code, m[1]-1)
			args := splitArgs(inner)
			if len(args) == 0 {
				continue
			}
			add(outCall{Kind: "http", Client: clientName(recv, sy, s.lang), Method: method, Addr: s.renderAddr(args[0], sy), Recv: lastSegment(recv)}, m[0])
		}
		for _, loc := range javaHTTPReq.FindAllStringIndex(code, -1) {
			stmt := s.statementFrom(loc[0])
			addr := ""
			if u := rx(`(?:newBuilder|\.uri)\s*\(\s*(?:URI\s*\.\s*create|new\s+URI|URI)\s*\(`).FindStringIndex(stmt); u != nil {
				inner, _ := balanced(stmt, u[1]-1)
				addr = inner
			}
			if addr == "" {
				continue
			}
			method := "GET"
			if mm := rx(`\.\s*(GET|POST|PUT|DELETE|PATCH|HEAD)\s*\(|\.\s*method\s*\(\s*"(\w+)"`).FindStringSubmatch(stmt); mm != nil {
				method = mm[1] + mm[2]
			}
			add(outCall{Kind: "http", Client: "java.net.http", Method: method, Addr: s.renderAddr(addr, sy)}, loc[0])
		}
		for _, loc := range okHTTPReq.FindAllStringIndex(code, -1) {
			stmt := s.statementFrom(loc[0])
			u := rx(`\.\s*url\s*\(`).FindStringIndex(stmt)
			if u == nil {
				continue
			}
			inner, _ := balanced(stmt, u[1]-1)
			method := "GET"
			if mm := rx(`\.\s*(post|put|delete|patch|head)\s*\(`).FindStringSubmatch(stmt); mm != nil {
				method = strings.ToUpper(mm[1])
			}
			add(outCall{Kind: "http", Client: "okhttp", Method: method, Addr: s.renderAddr(inner, sy)}, loc[0])
		}
		for _, m := range apacheReq.FindAllStringSubmatchIndex(code, -1) {
			inner, _ := balanced(code, m[1]-1)
			if args := splitArgs(inner); len(args) > 0 {
				add(outCall{Kind: "http", Client: "apache-httpclient", Method: strings.ToUpper(code[m[2]:m[3]]), Addr: s.renderAddr(args[0], sy)}, m[0])
			}
		}
		for _, loc := range jaxrsTarget.FindAllStringIndex(code, -1) {
			inner, end := balanced(code, loc[1]-1)
			args := splitArgs(inner)
			if len(args) != 1 {
				continue
			}
			rest := code[end:min(len(code), end+400)]
			if !strings.Contains(rest, ".request(") {
				continue
			}
			addr := s.renderAddr(args[0], sy)
			for _, p := range rx(`\.\s*path\s*\(\s*("[^"]*")\s*\)`).FindAllStringSubmatch(rest[:strings.Index(rest, ".request(")], -1) {
				addr = strings.TrimRight(addr, "/") + "/" + strings.TrimLeft(unquote(p[1]), "/")
			}
			method := "GET"
			if mm := rx(`\.\s*(get|post|put|delete|patch|head|method)\s*\(\s*(?:"(\w+)")?`).FindStringSubmatch(rest[strings.Index(rest, ".request("):]); mm != nil {
				method = strings.ToUpper(mm[1])
				if mm[1] == "method" && mm[2] != "" {
					method = strings.ToUpper(mm[2])
				}
			}
			add(outCall{Kind: "http", Client: "jax-rs-client", Method: method, Addr: addr}, loc[0])
		}
		for _, m := range unirestCall.FindAllStringSubmatchIndex(code, -1) {
			inner, _ := balanced(code, m[1]-1)
			if args := splitArgs(inner); len(args) > 0 {
				add(outCall{Kind: "http", Client: "unirest", Method: strings.ToUpper(code[m[2]:m[3]]), Addr: s.renderAddr(args[0], sy)}, m[0])
			}
		}
	case s.lang == langCSharp:
		for _, m := range csRequestMsg.FindAllStringSubmatchIndex(code, -1) {
			open := strings.Index(code[m[0]:], "(") + m[0]
			inner, _ := balanced(code, open)
			args := splitArgs(inner)
			if len(args) < 2 {
				continue
			}
			method := ""
			if m[2] >= 0 {
				method = code[m[2]:m[3]]
			} else {
				method = code[m[4]:m[5]]
			}
			add(outCall{Kind: "http", Client: "httpclient", Method: strings.ToUpper(method), Addr: s.renderAddr(args[1], sy)}, m[0])
		}
		for _, loc := range restSharpReq.FindAllStringIndex(code, -1) {
			inner, _ := balanced(code, loc[1]-1)
			args := splitArgs(inner)
			if len(args) == 0 {
				continue
			}
			method := "GET"
			if len(args) > 1 {
				if mm := methodLiteral(args[1]); mm != "" {
					method = mm
				}
			}
			add(outCall{Kind: "http", Client: "restsharp", Method: method, Addr: s.renderAddr(args[0], sy)}, loc[0])
		}
	case jsLike[s.lang]:
		for _, m := range jsFetch.FindAllStringSubmatchIndex(code, -1) {
			open := m[1] - 1
			inner, _ := balanced(code, open)
			args := splitArgs(inner)
			if len(args) == 0 {
				continue
			}
			method := "GET"
			if len(args) > 1 {
				if mm := rx(`\bmethod\s*:\s*['"` + "`" + `](\w+)`).FindStringSubmatch(args[1]); mm != nil {
					method = strings.ToUpper(mm[1])
				}
			}
			rendered := s.renderAddr(args[0], sy)
			if !looksLikeAddr(rendered, args[0]) {
				continue
			}
			add(outCall{Kind: "http", Client: strings.TrimPrefix(code[m[2]:m[3]], "$"), Method: method, Addr: rendered}, m[2])
		}
		for _, loc := range jsAxiosConfig.FindAllStringIndex(code, -1) {
			body, _ := balanced(code, loc[1]-1)
			addr := objectKey(body, "url")
			if addr == "" {
				continue
			}
			method := "GET"
			if mm := methodLiteral(objectKey(body, "method")); mm != "" {
				method = mm
			}
			add(outCall{Kind: "http", Client: "axios", Method: method, Addr: s.renderAddr(addr, sy)}, loc[0])
		}
	case s.lang == langPython:
		for _, loc := range pyURLOpen.FindAllStringIndex(code, -1) {
			if loc[0] > 0 && (code[loc[0]-1] == '.' && !strings.HasSuffix(code[max(0, loc[0]-20):loc[0]], "urllib.request.")) {
				continue
			}
			inner, _ := balanced(code, loc[1]-1)
			args := splitArgs(inner)
			if len(args) == 0 {
				continue
			}
			rendered := s.renderAddr(args[0], sy)
			if !looksLikeAddr(rendered, args[0]) {
				continue
			}
			add(outCall{Kind: "http", Client: "urllib", Method: "GET", Addr: rendered}, loc[0])
		}
		for _, m := range celeryCall.FindAllStringSubmatchIndex(code, -1) {
			name := code[m[2]:m[3]]
			if notCalls[name] {
				continue
			}
			add(outCall{Kind: "message", Client: "celery", Method: "TASK", Target: name, Addr: name}, m[0])
		}
	case s.lang == langGo:
		for _, m := range goHTTPCall.FindAllStringSubmatchIndex(code, -1) {
			inner, _ := balanced(code, m[1]-1)
			if args := splitArgs(inner); len(args) > 0 {
				method := strings.ToUpper(code[m[2]:m[3]])
				if method == "POSTFORM" {
					method = "POST"
				}
				add(outCall{Kind: "http", Client: "net/http", Method: method, Addr: s.renderAddr(args[0], sy)}, m[0])
			}
		}
		for _, loc := range goNewRequest.FindAllStringIndex(code, -1) {
			inner, _ := balanced(code, loc[1]-1)
			args := splitArgs(inner)
			if strings.HasSuffix(code[loc[0]:loc[1]], "WithContext(") || len(args) == 4 {
				args = args[1:]
			}
			if len(args) < 2 {
				continue
			}
			method := methodLiteral(args[0])
			if method == "" {
				method = "ANY"
			}
			add(outCall{Kind: "http", Client: "net/http", Method: method, Addr: s.renderAddr(args[1], sy)}, loc[0])
		}
		for _, m := range restyChain.FindAllStringSubmatchIndex(code, -1) {
			inner, _ := balanced(code, m[1]-1)
			if args := splitArgs(inner); len(args) > 0 {
				add(outCall{Kind: "http", Client: "resty", Method: strings.ToUpper(code[m[4]:m[5]]), Addr: s.renderAddr(args[0], sy)}, m[0])
			}
		}
	case s.lang == langPHP:
		for _, loc := range laravelHTTP.FindAllStringIndex(code, -1) {
			stmt := s.statementFrom(loc[0])
			mm := rx(`(?:::|->)\s*(get|post|put|patch|delete|head|send)\s*\(`).FindStringSubmatchIndex(stmt)
			if mm == nil {
				continue
			}
			inner, _ := balanced(stmt, mm[1]-1)
			args := splitArgs(inner)
			method := strings.ToUpper(stmt[mm[2]:mm[3]])
			if method == "SEND" && len(args) > 1 {
				method = methodLiteral(args[0])
				args = args[1:]
			}
			if len(args) == 0 {
				continue
			}
			addr := s.renderAddr(args[0], sy)
			if b := rx(`baseUrl\s*\(`).FindStringIndex(stmt); b != nil {
				bi, _ := balanced(stmt, b[1]-1)
				addr = strings.TrimRight(s.renderAddr(bi, sy), "/") + "/" + strings.TrimLeft(addr, "/")
			}
			add(outCall{Kind: "http", Client: "laravel-http", Method: method, Addr: addr}, loc[0])
		}
		for _, loc := range phpCurl.FindAllStringIndex(code, -1) {
			start := loc[1]
			if code[loc[1]-1] == '(' {
				start = loc[1] - 1
			} else {
				start = loc[1]
			}
			var arg string
			if code[loc[1]-1] == '(' {
				inner, _ := balanced(code, start)
				if args := splitArgs(inner); len(args) > 0 {
					arg = args[0]
				}
			} else {
				end := strings.IndexAny(code[start:], ");")
				if end > 0 {
					arg = code[start : start+end]
				}
			}
			if strings.TrimSpace(arg) == "" {
				continue
			}
			add(outCall{Kind: "http", Client: "curl", Method: "ANY", Addr: s.renderAddr(arg, sy)}, loc[0])
		}
	}

	// Table rules: messaging, datastores, mail, LDAP, storage, file transfer,
	// gRPC, GraphQL, SOAP.
	for i, r := range outRules {
		if !r.langs[s.lang] || ruleHints[i] != "" && !strings.Contains(code, ruleHints[i]) {
			continue
		}
		for _, m := range r.re.FindAllStringSubmatchIndex(code, -1) {
			if r.gate != "" && m[2] >= 0 {
				recv := normalRecv(strings.Join(strings.Fields(code[m[2]:m[3]]), ""))
				if !gateMatches(r.gate, recv, sy) {
					continue
				}
			} else if r.gate != "" {
				continue
			}
			c := outCall{Kind: r.kind, Client: r.client, Method: r.method}
			last := code[m[1]-1]
			var body string
			if last == '(' || last == '{' {
				body, _ = balanced(code, m[1]-1)
			}
			args := splitArgs(body)
			switch {
			case r.key != "":
				v := objectKey(body, r.key)
				if v == "" && r.arg >= 0 && r.arg < len(positional(args)) {
					v = positional(args)[r.arg]
				}
				if v != "" {
					c.Addr = s.renderAddr(v, sy)
				}
			case r.arg >= 0 && r.arg < len(args):
				c.Addr = s.renderAddr(args[r.arg], sy)
			}
			// A captured name is the target: a gRPC service, a message type.
			if g := len(m) / 2; g > 1 {
				lastGroup := m[len(m)-2:]
				if lastGroup[0] >= 0 && (r.gate == "" || g > 3) {
					c.Target = code[lastGroup[0]:lastGroup[1]]
					c.Target = c.Target[strings.LastIndexByte(c.Target, '\\')+1:]
				}
			}
			if c.Kind == "message" && c.Addr == "" && c.Target != "" {
				c.Addr = c.Target
			}
			add(c, m[0])
		}
	}
	return out, bases
}

// declaredClients are the methods of an HTTP client interface, each a call
// with its method and path: Feign, MicroProfile Rest Client, Spring's
// HttpExchange, Retrofit, Refit.
func (s *source) declaredClients() []outCall {
	var out []outCall
	decls := s.decls()
	type client struct {
		name, base, target, path, kind string
		line                           int
	}
	var clients []client
	for _, d := range decls {
		if !d.Class {
			continue
		}
		for _, m := range d.Marks {
			c := client{name: d.Name, line: d.Line}
			switch m.Name {
			case "FeignClient":
				c.kind, c.target = "feign", m.arg("name", "value", "contextId")
				c.base = named(m.Args, "url")
				c.path = named(m.Args, "path")
			case "RegisterRestClient":
				c.kind, c.target = "microprofile", named(m.Args, "configKey")
				c.base = named(m.Args, "baseUri")
				if p := d.has("Path"); p != nil {
					c.path = p.arg("value")
				}
			case "HttpExchange":
				c.kind = "http-exchange"
				c.base = m.arg("url", "value")
			default:
				continue
			}
			clients = append(clients, c)
		}
	}
	enclosingClient := func(line int) *client {
		var found *client
		for i := range clients {
			if clients[i].line <= line {
				found = &clients[i]
			}
		}
		return found
	}
	isInterface := func(line int) bool {
		for _, t := range s.typeDecls() {
			if t.Line <= line {
				if strings.Contains(" "+t.Signature+" ", " interface ") {
					return true
				}
			}
		}
		return false
	}
	for _, d := range decls {
		if d.Class {
			continue
		}
		c := enclosingClient(d.Line)
		for _, m := range d.Marks {
			method, p := "", ""
			kind := ""
			switch {
			case c != nil && (c.kind == "feign") && (strings.HasSuffix(m.Name, "Mapping")):
				if m.Name == "RequestMapping" {
					method = requestMethod(m.Args)
				} else {
					method = strings.ToUpper(strings.TrimSuffix(m.Name, "Mapping"))
				}
				p, kind = m.arg("value", "path"), c.kind
			case c != nil && c.kind == "microprofile" && jaxrsVerbs[m.Name]:
				method, kind = m.Name, c.kind
				if pm := d.has("Path"); pm != nil {
					p = pm.arg("value")
				}
			case c != nil && c.kind == "http-exchange" && strings.HasSuffix(m.Name, "Exchange"):
				method = strings.ToUpper(strings.TrimSuffix(m.Name, "Exchange"))
				if method == "HTTP" {
					method = strings.ToUpper(named(m.Args, "method"))
				}
				p, kind = m.arg("url", "value"), c.kind
			case jvm[s.lang] && jaxrsVerbs[m.Name] && len(m.Args) > 0:
				// Retrofit: @GET("users/{id}") on an interface method.
				method, p, kind = m.Name, m.arg("value"), "retrofit"
			case s.lang == langCSharp && refitVerbs[m.Name] && isInterface(d.Line):
				method, p, kind = strings.ToUpper(m.Name), m.arg("value"), "refit"
			default:
				continue
			}
			call := outCall{Kind: "http", Client: kind, Method: method, Declared: true, Line: m.Line}
			call.Class = s.typeAtLine(d.Line)
			if c != nil && kind != "retrofit" && kind != "refit" {
				call.Target, call.Base = c.target, c.base
				p = joinRoute(c.path, p)
			}
			call.Addr = p
			if call.Addr == "" {
				call.Addr = "/"
			}
			if r := resilienceOf(d.Marks); r != "" {
				call.Resilience = r
			}
			out = append(out, call)
		}
	}
	return out
}

var refitVerbs = map[string]bool{"Get": true, "Post": true, "Put": true, "Delete": true, "Patch": true, "Head": true, "Options": true}

func (s *source) typeAtLine(line int) string {
	name := ""
	for _, t := range s.typeDecls() {
		if t.Line <= line {
			name = t.Name
		}
	}
	return name
}

// clientBases are the base addresses a file binds to client variables and
// beans: axios.create({baseURL}), WebClient.builder().baseUrl(x), a
// HttpClient's BaseAddress, Guzzle's base_uri, resty's SetBaseURL,
// httpx.Client(base_url=), AddHttpClient<T>(c => c.BaseAddress = ...).
func (s *source) clientBases(sy *symbols) []clientBase {
	code := s.code
	var out []clientBase
	if !clientBaseHint.MatchString(code) {
		return nil
	}
	add := func(name, expr string, at int) {
		if name == "" || strings.TrimSpace(expr) == "" {
			return
		}
		out = append(out, clientBase{Name: name, Addr: s.renderAddr(expr, sy), Resilience: resilienceIn(s.statement(at)), Line: s.line(at)})
	}
	for _, m := range rx(`\b([A-Za-z_$][\w$]*)\s*(?::\s*[\w<>.]+\s*)?=\s*axios\s*\.\s*create\s*\(\s*\{`).FindAllStringSubmatchIndex(code, -1) {
		body, _ := balanced(code, m[1]-1)
		add(code[m[2]:m[3]], objectKey(body, "baseURL"), m[0])
	}
	if m := rx(`axios\s*\.\s*defaults\s*\.\s*baseURL\s*=\s*([^;\n]+)`).FindStringSubmatchIndex(code); m != nil {
		add("axios", code[m[2]:m[3]], m[0])
	}
	for _, m := range rx(`\b([A-Za-z_]\w*)\s*(?:=|:=)\s*(?:WebClient|RestClient)\s*\.\s*(?:builder|create)\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		stmt := s.statementFrom(m[0])
		if b := rx(`(?:baseUrl|create)\s*\(`).FindStringIndex(stmt); b != nil {
			inner, _ := balanced(stmt, b[1]-1)
			add(code[m[2]:m[3]], inner, m[0])
		}
	}
	// A @Bean method that returns a client built with a base: the bean is
	// injected by its method's name.
	for _, d := range s.decls() {
		if d.Class || d.has("Bean") == nil {
			continue
		}
		body := s.body(d.Line)
		if b := rx(`(?:\.baseUrl|WebClient\s*\.\s*create|\.rootUri|DefaultUriBuilderFactory)\s*\(`).FindStringIndex(body); b != nil {
			inner, _ := balanced(body, b[1]-1)
			if inner != "" {
				out = append(out, clientBase{Name: d.Name, Addr: s.renderAddr(inner, sy), Resilience: resilienceIn(body), Line: d.Line})
			}
		}
	}
	for _, m := range rx(`\b([A-Za-z_]\w*)\s*\.\s*BaseAddress\s*=\s*new\s+Uri\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := balanced(code, m[1]-1)
		add(code[m[2]:m[3]], inner, m[0])
	}
	// AddHttpClient<IOrders, Orders>(c => c.BaseAddress = new Uri(...)),
	// AddHttpClient("orders", ...), AddRefitClient<IOrders>().ConfigureHttpClient(...).
	for _, m := range rx(`\bAdd(?:HttpClient|RefitClient)\s*(?:<\s*([\w.]+)\s*(?:,\s*([\w.]+)\s*)?>)?\s*\(\s*(?:"([^"]+)")?`).FindAllStringSubmatchIndex(code, -1) {
		stmt := s.statementFrom(m[0])
		b := rx(`BaseAddress\s*=\s*new\s+Uri\s*\(`).FindStringIndex(stmt)
		expr := ""
		if b != nil {
			expr, _ = balanced(stmt, b[1]-1)
		}
		res := resilienceIn(stmt)
		for _, g := range []int{2, 4, 6} {
			if m[g] >= 0 {
				name := code[m[g]:m[g+1]]
				out = append(out, clientBase{Name: name[strings.LastIndexByte(name, '.')+1:], Addr: s.renderAddr(expr, sy), Resilience: res, Line: s.line(m[0])})
			}
		}
	}
	for _, m := range rx(`\bRestService\s*\.\s*For\s*<\s*(\w+)\s*>\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := balanced(code, m[1]-1)
		if args := splitArgs(inner); len(args) > 0 {
			add(code[m[2]:m[3]], args[0], m[0])
		}
	}
	for _, m := range rx(`\b([A-Za-z_]\w*)\s*=\s*(?:new\s+)?(?:RestClient)\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := balanced(code, m[1]-1)
		if args := splitArgs(inner); len(args) > 0 && s.lang == langCSharp {
			add(code[m[2]:m[3]], args[0], m[0])
		}
	}
	// Python: httpx.Client(base_url=...), requests sessions don't carry one.
	for _, m := range rx(`\b([a-z_]\w*)\s*=\s*httpx\s*\.\s*(?:Async)?Client\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := balanced(code, m[1]-1)
		add(code[m[2]:m[3]], objectKey(inner, "base_url"), m[0])
	}
	// PHP: new Client(['base_uri' => ...]), HttpClient::createForBaseUri(...).
	for _, m := range rx(`\$(?:this->)?(\w+)\s*=\s*new\s+\\?(?:GuzzleHttp\\)?Client\s*\(\s*\[`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := balanced(code, m[1]-1)
		add(code[m[2]:m[3]], objectKey(inner, "base_uri"), m[0])
	}
	for _, m := range rx(`\$(?:this->)?(\w+)\s*=\s*\\?(?:[\w\\]*\\)?HttpClient::createForBaseUri\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := balanced(code, m[1]-1)
		if args := splitArgs(inner); len(args) > 0 {
			add(code[m[2]:m[3]], args[0], m[0])
		}
	}
	// Go: resty.New().SetBaseURL(...).
	for _, m := range rx(`\b([a-zA-Z_]\w*)\s*(?::=|=)\s*resty\s*\.\s*New\s*\(\s*\)`).FindAllStringSubmatchIndex(code, -1) {
		stmt := s.statementFrom(m[0])
		if b := rx(`SetBaseURL\s*\(`).FindStringIndex(stmt); b != nil {
			inner, _ := balanced(stmt, b[1]-1)
			add(code[m[2]:m[3]], inner, m[0])
		}
	}
	return out
}

// isHTTPClient is whether a receiver is an HTTP client: by its declared
// type, a module that is one, a variable a base was bound to, or a name
// nothing else goes by.
func (s *source) isHTTPClient(recv string, sy *symbols, bases []clientBase) bool {
	if recv == "" || recv == "this" {
		return false
	}
	last := lastSegment(recv)
	if t, ok := sy.vars[last]; ok {
		g := genericless(t)
		if httpTypes.MatchString(g) || httpTypes.MatchString(g[strings.LastIndexAny(g, `.\`)+1:]) && g != "Client" {
			return g != "Client" || s.lang == langPHP && strings.Contains(s.raw, "GuzzleHttp")
		}
		if strings.Contains(strings.ToLower(g), "redis") || strings.Contains(strings.ToLower(g), "cache") || strings.Contains(strings.ToLower(g), "repository") {
			return false
		}
	}
	for _, b := range bases {
		if b.Name == last {
			return true
		}
	}
	if httpModule[recv] {
		switch recv {
		case "Http":
			return s.lang == langPHP
		case "requests", "httpx":
			return s.lang == langPython
		}
		return true
	}
	if recv == "http" && s.lang == langGo {
		return false // net/http's package functions are read on their own
	}
	if httpNames.MatchString(last) && !rx(`(?i)request$|response$|context$|handler$|server$|router$|app$`).MatchString(last) {
		_, typed := sy.vars[last]
		return !typed || httpTypes.MatchString(genericless(sy.vars[last]))
	}
	return false
}

func gateMatches(gate, recv string, sy *symbols) bool {
	re := rx(gate)
	last := lastSegment(recv)
	if re.MatchString(last) {
		return true
	}
	if t, ok := sy.vars[last]; ok && re.MatchString(genericless(t)) {
		return true
	}
	return false
}

func lastSegment(recv string) string {
	r := strings.NewReplacer("->", ".", "?.", ".", "::", ".").Replace(recv)
	return r[strings.LastIndexByte(r, '.')+1:]
}

// clientName is the library a call goes through, from the receiver's type
// or name.
func clientName(recv string, sy *symbols, lang string) string {
	last := lastSegment(recv)
	t := genericless(sy.vars[last])
	if t == "" {
		t = recv
	}
	lt := strings.ToLower(t)
	switch {
	case strings.Contains(lt, "resttemplate"):
		return "resttemplate"
	case strings.Contains(lt, "webclient"):
		return "webclient"
	case strings.Contains(lt, "restclient") && lang != langCSharp:
		return "restclient"
	case strings.Contains(lt, "okhttp"):
		return "okhttp"
	case strings.Contains(lt, "httpservice"):
		return "nestjs-http"
	case strings.Contains(lt, "axios") || recv == "axios":
		return "axios"
	case strings.Contains(lt, "httpx"):
		return "httpx"
	case strings.Contains(lt, "aiohttp") || strings.Contains(lt, "clientsession"):
		return "aiohttp"
	case strings.Contains(lt, "requests") || lt == "session" && lang == langPython:
		return "requests"
	case strings.Contains(lt, "resty"):
		return "resty"
	case lang == langPHP && (strings.Contains(lt, "httpclientinterface")):
		return "symfony-http-client"
	case lang == langPHP:
		return "guzzle"
	case lang == langCSharp && strings.Contains(lt, "restclient"):
		return "restsharp"
	case lang == langCSharp:
		return "httpclient"
	case lang == langJS && (strings.Contains(lt, "httpclient") || last == "http"):
		return "angular-http"
	case lang == langKotlin && strings.Contains(lt, "httpclient"):
		return "ktor-client"
	case lang == langJava && strings.Contains(lt, "httpclient"):
		return "java.net.http"
	}
	if httpModule[recv] {
		return recv
	}
	return "http"
}

// httpVerbOf is the method a client member sends: getForObject is GET,
// PostAsJsonAsync is POST.
func httpVerbOf(member string) string {
	m := verbMember.FindStringSubmatch(member)
	if m == nil {
		return ""
	}
	v := strings.ToUpper(m[1] + m[2] + m[3])
	if v == "DEL" {
		v = "DELETE"
	}
	return v
}

var methodWord = rx(`(?i)^(?:HttpMethod\s*\.\s*|Method\s*\.\s*|http\s*\.\s*Method|["'])?(get|post|put|patch|delete|head|options)["']?$`)

// methodLiteral is the HTTP method an argument names: "GET", HttpMethod.Post,
// http.MethodGet, Method.Get.
func methodLiteral(arg string) string {
	a := strings.TrimSpace(arg)
	if m := methodWord.FindStringSubmatch(a); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}

// objectKey is the value a named key holds in an object literal, keyword
// arguments, a Go struct literal or a PHP array: `baseURL: x`, `base_url=x`,
// `Topic: x`, `'base_uri' => x`.
func objectKey(body, key string) string {
	for _, part := range splitArgs(body) {
		t := strings.TrimSpace(part)
		for _, sep := range []string{"=>", ":", "="} {
			i := strings.Index(t, sep)
			if i <= 0 {
				continue
			}
			k := strings.Trim(strings.TrimSpace(t[:i]), `"'`)
			if strings.EqualFold(k, key) {
				return strings.TrimSpace(t[i+len(sep):])
			}
			break
		}
	}
	return ""
}

// looksLikeAddr is whether a rendered address can be an HTTP address: a
// path, a URL, or something built from configuration. `get(key)` on a
// generic client with a bare variable is not enough.
func looksLikeAddr(rendered, raw string) bool {
	r := strings.TrimSpace(rendered)
	switch {
	case r == "":
		return false
	case strings.HasPrefix(r, "/"), strings.Contains(r, "://"), strings.HasPrefix(r, "{config:"), strings.HasPrefix(r, "{env:"), strings.HasPrefix(r, "{prop:"):
		return true
	case strings.HasPrefix(r, "{var:"):
		v := strings.ToLower(r)
		return strings.Contains(v, "url") || strings.Contains(v, "uri") || strings.Contains(v, "endpoint") || strings.Contains(v, "path") || strings.Contains(v, "/") || strings.Contains(v, "host") || strings.Contains(v, "address")
	}
	// A relative path without its slash: "users/{id}", "api/orders".
	_, literal := literalText(strings.TrimSpace(raw))
	return rx(`^[a-z][\w.-]*/[\w{}./:-]*$`).MatchString(r) && literal
}

// statement is the text of the statement around an offset, for reading a
// call's options: from the line's start to the end of the statement.
func (s *source) statement(at int) string {
	line := s.line(at)
	start := s.lines[line-1]
	return s.statementFrom(start)
}

// statementFrom is the text from an offset to the end of its statement: the
// first `;` or blank line at depth zero, at most 1,500 bytes.
func (s *source) statementFrom(at int) string {
	code := s.code
	depth := 0
	limit := min(len(code), at+1500)
	for i := at; i < limit; i++ {
		switch code[i] {
		case '"', '\'', '`':
			i = skipString(code, i, s.lang) - 1
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
			if depth < 0 {
				return code[at:i]
			}
		case ';':
			if depth == 0 {
				return code[at:i]
			}
		case '\n':
			if depth == 0 && i+1 < len(code) && (code[i+1] == '\n' || s.lang == langPython || s.lang == langGo && !strings.HasSuffix(strings.TrimSpace(code[at:i]), ".")) {
				next := strings.TrimSpace(code[i+1 : min(len(code), i+80)])
				if !strings.HasPrefix(next, ".") && !strings.HasPrefix(next, "->") {
					return code[at:i]
				}
			}
		}
	}
	return code[at:limit]
}

func resilienceIn(text string) string {
	var r []string
	if timeoutArg.MatchString(text) {
		r = append(r, "timeout")
	}
	if retryArg.MatchString(text) {
		r = append(r, "retry")
	}
	if circuitArg.MatchString(text) {
		r = append(r, "circuit_breaker")
	}
	return strings.Join(r, ",")
}

func resilienceOf(marks []mark) string {
	set := map[string]bool{}
	for _, m := range marks {
		if r, ok := resilienceMark[m.Name]; ok {
			set[r] = true
		}
	}
	var out []string
	for _, k := range []string{"timeout", "retry", "circuit_breaker"} {
		if set[k] {
			out = append(out, k)
		}
	}
	return strings.Join(out, ",")
}

func mergeResilience(a, b string) string {
	set := map[string]bool{}
	for _, x := range strings.Split(a+","+b, ",") {
		if x != "" {
			set[x] = true
		}
	}
	var out []string
	for _, k := range []string{"timeout", "retry", "circuit_breaker"} {
		if set[k] {
			out = append(out, k)
		}
	}
	return strings.Join(out, ",")
}

var (
	envRead    = regexp.MustCompile(`^(?:process\s*\.\s*env|import\s*\.\s*meta\s*\.\s*env)\s*(?:\.\s*([A-Za-z_]\w*)|\[\s*['"]([^'"]+)['"]\s*\])$|^(?:os\s*\.\s*(?:getenv|Getenv|environ\s*\.\s*get)|System\s*\.\s*getenv|Environment\s*\.\s*GetEnvironmentVariable|getenv|env|Deno\s*\.\s*env\s*\.\s*get)\s*\(\s*['"]([^'"]+)['"]|^os\s*\.\s*environ\s*\[\s*['"]([^'"]+)['"]\s*\]$`)
	cfgRead    = regexp.MustCompile(`^(?:[\w.]*[cC]onfig(?:uration)?|_config|_configuration|[\w.]*[sS]ettings|[\w.]*[eE]nv(?:ironment)?)\s*\[\s*["']([^"']+)["']\s*\]$|\.\s*(?:GetValue\s*(?:<[^<>]*>)?|getProperty|getRequiredProperty|GetString|getString|get|GetConnectionString)\s*\(\s*["']([^"']+)["']|^config\s*\(\s*['"]([^'"]+)['"]|^viper\s*\.\s*Get\w*\s*\(\s*"([^"]+)"`)
	djangoSet  = regexp.MustCompile(`^settings\s*\.\s*([A-Z][A-Z0-9_]*)$`)
	fmtCall    = regexp.MustCompile(`^(?:String\s*\.\s*format|fmt\s*\.\s*Sprintf|sprintf|string\s*\.\s*Format|String\s*\.\s*Format|format)\s*\(`)
	wrapCall   = regexp.MustCompile(`^(?:URI\s*\.\s*create|new\s+URI|new\s+Uri|new\s+URL|URI|URL|Uri\s*\.\s*parse|url\s*\.\s*Parse|URI\s*\.\s*parse|java\.net\.URI\s*\.\s*create|UriComponentsBuilder\s*\.\s*fromHttpUrl|UriComponentsBuilder\s*\.\s*fromUriString|urljoin|urllib\.parse\.urljoin|Path\s*\.\s*Combine|path\s*\.\s*join|url\s*\.\s*JoinPath|Url\s*\.\s*Combine)\s*\(`)
	interp     = regexp.MustCompile(`\$\{([^{}]+)\}|\{\$?([A-Za-z_][\w.>-]*(?:\([^(){}]*\))?)\}|\$([A-Za-z_][\w]*)`)
	pyFmtCall  = regexp.MustCompile(`^(["'][^"']*["'])\s*\.\s*format\s*\(`)
	stripCalls = regexp.MustCompile(`\.\s*(?:toString|toUriString|build|toUri|encode|expand|buildAndExpand|trim|strip|rstrip|TrimEnd|TrimStart|Trim|String)\s*\([^()]*\)$`)
)

// renderAddr renders an address expression: literals as they are, every
// other part as a marker the linker can resolve.
func (s *source) renderAddr(expr string, sy *symbols) string {
	return s.render(strings.TrimSpace(expr), sy, 0)
}

func (s *source) render(e string, sy *symbols, depth int) string {
	if e == "" || depth > 6 {
		return ""
	}
	e = strings.TrimSpace(stripCalls.ReplaceAllString(strings.TrimSpace(e), ""))
	for strings.HasPrefix(e, "(") && strings.HasSuffix(e, ")") {
		inner, end := balanced(e, 0)
		if end != len(e) {
			break
		}
		e = strings.TrimSpace(inner)
	}
	// Wrappers that build a URI from a string: URI.create(x), new URL(p, base).
	if loc := wrapCall.FindStringIndex(e); loc != nil {
		inner, end := balanced(e, loc[1]-1)
		if end == len(e) || strings.HasPrefix(strings.TrimSpace(e[end:]), ".") {
			args := splitArgs(inner)
			switch len(args) {
			case 0:
				return ""
			case 1:
				return s.render(args[0], sy, depth+1)
			}
			name := e[:loc[1]-1]
			if strings.Contains(name, "URL") && !strings.Contains(name, "URI") {
				// new URL(path, base)
				return joinAddr(s.render(args[1], sy, depth+1), s.render(args[0], sy, depth+1))
			}
			out := s.render(args[0], sy, depth+1)
			for _, a := range args[1:] {
				out = joinAddr(out, s.render(a, sy, depth+1))
			}
			return out
		}
	}
	// Formatting: String.format("%s/orders/%d", base, id), "{}/x".format(b).
	if loc := fmtCall.FindStringIndex(e); loc != nil {
		inner, _ := balanced(e, loc[1]-1)
		args := splitArgs(inner)
		if len(args) > 0 {
			return s.format(unquote(args[0]), args[1:], sy, depth)
		}
	}
	if m := pyFmtCall.FindStringSubmatchIndex(e); m != nil {
		inner, _ := balanced(e, m[1]-1)
		return s.format(unquote(e[m[2]:m[3]]), splitArgs(inner), sy, depth)
	}
	if i := strings.Index(e, `" % `); i > 0 || strings.Index(e, `' % `) > 0 {
		if i <= 0 {
			i = strings.Index(e, `' % `)
		}
		rest := strings.TrimSpace(e[i+3:])
		rest = strings.Trim(rest, "()")
		return s.format(unquote(e[:i+1]), splitArgs(rest), sy, depth)
	}
	// Concatenation: + everywhere, . in PHP.
	parts := splitTop(e, "+")
	if len(parts) == 1 && s.lang == langPHP {
		parts = splitTop(e, ".")
	}
	if len(parts) > 1 {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(s.render(p, sy, depth+1))
		}
		return b.String()
	}
	// A string, interpolations included.
	if lit, ok := literalText(e); ok {
		return s.interpolate(lit, sy, depth)
	}
	return s.token(e, sy, depth)
}

// literalText is the text of a string literal of any kind: "x", 'x', `x`,
// f"x", $"x", @"x", r"x", """x""".
func literalText(e string) (string, bool) {
	t := strings.TrimLeft(e, "fFrRbBuU$@")
	if len(t) < 2 {
		return "", false
	}
	q := t[0]
	if q != '"' && q != '\'' && q != '`' {
		return "", false
	}
	if strings.HasPrefix(t, `"""`) && strings.HasSuffix(t, `"""`) && len(t) >= 6 {
		return t[3 : len(t)-3], true
	}
	if t[len(t)-1] != q {
		return "", false
	}
	return t[1 : len(t)-1], true
}

// interpolate renders the expressions inside a literal: `${base}/x`,
// f"{base}/x", "$base/x", "{$this->base}/x".
func (s *source) interpolate(lit string, sy *symbols, depth int) string {
	if !strings.ContainsAny(lit, "{$") {
		return lit
	}
	// An OpenAPI-style template `/orders/{id}` is a literal path parameter,
	// not an interpolation, unless the language interpolates braces.
	braces := s.lang == langPython || s.lang == langCSharp || s.lang == langPHP
	return interp.ReplaceAllStringFunc(lit, func(m string) string {
		g := interp.FindStringSubmatch(m)
		switch {
		case g[1] != "":
			return s.token(strings.TrimSpace(g[1]), sy, depth+1)
		case g[2] != "":
			if !braces && !strings.HasPrefix(m, "{$") {
				return m
			}
			return s.token(strings.TrimSpace(strings.TrimPrefix(g[2], "$")), sy, depth+1)
		case g[3] != "":
			if s.lang == langKotlin || s.lang == langPHP || s.lang == langDart || s.lang == langGo {
				return s.token(g[3], sy, depth+1)
			}
		}
		return m
	})
}

// format fills %s, %d, %v, {} and {0} in a format string with its rendered
// arguments.
func (s *source) format(f string, args []string, sy *symbols, depth int) string {
	i := 0
	next := func() string {
		if i >= len(args) {
			return "{}"
		}
		a := s.render(args[i], sy, depth+1)
		i++
		return a
	}
	return rx(`%[-+ #0-9.]*[sdvqfx]|\{\d*\}|\{[a-z_]\w*\}`).ReplaceAllStringFunc(f, func(string) string { return next() })
}

// token renders one non-literal part of an address.
func (s *source) token(e string, sy *symbols, depth int) string {
	e = strings.TrimSpace(e)
	if e == "" {
		return ""
	}
	if m := envRead.FindStringSubmatch(e); m != nil {
		return "{env:" + m[1] + m[2] + m[3] + m[4] + "}"
	}
	if m := cfgRead.FindStringSubmatch(e); m != nil {
		key := m[1] + m[2] + m[3] + m[4]
		if strings.Contains(e, "GetConnectionString") {
			key = "ConnectionStrings:" + key
		}
		return "{config:" + key + "}"
	}
	if m := djangoSet.FindStringSubmatch(e); m != nil {
		return "{config:" + m[1] + "}"
	}
	if a, ok := sy.addrs[lastSegment(normalRecv(strings.Join(strings.Fields(e), "")))]; ok && depth < 5 {
		return a
	}
	name := normalRecv(strings.Join(strings.Fields(e), ""))
	name = strings.TrimSuffix(strings.TrimSuffix(name, "()"), "!!")
	if i := strings.IndexByte(name, '('); i > 0 {
		// A call that builds the address: named by what it calls.
		name = name[:i]
	}
	root, member := name, ""
	if i := strings.LastIndexAny(name, ".>"); i > 0 {
		root, member = strings.TrimRight(name[:i], "-"), name[i+1:]
		root = lastSegment(root)
	}
	if member == "" {
		if v, ok := sy.consts[root]; ok {
			return v
		}
		if k, ok := sy.keys[root]; ok {
			return "{config:" + k + "}"
		}
		if t, ok := sy.vars[root]; ok && strings.HasPrefix(strings.ToLower(genericless(t)), "uri") {
			return "{var:" + root + "}"
		}
		if !isIdentifier(strings.TrimPrefix(root, "$")) {
			return "{}"
		}
		return "{var:" + strings.TrimPrefix(root, "$") + "}"
	}
	// props.getBaseUrl(), options.Value.BaseUrl, settings.base_url: a
	// member of a configuration-bound type.
	if v, ok := sy.consts[member]; ok {
		return v
	}
	prop := strings.TrimPrefix(member, "get")
	if prop != member && len(prop) > 0 {
		prop = strings.ToLower(prop[:1]) + prop[1:]
	}
	if t, ok := sy.vars[root]; ok {
		typ := genericless(t)
		if (typ == "IOptions" || typ == "IOptionsMonitor" || typ == "IOptionsSnapshot") && strings.Contains(t, "<") {
			typ = strings.Trim(t[strings.IndexByte(t, '<')+1:], "<> ")
		}
		return "{prop:" + typ[strings.LastIndexAny(typ, `.\`)+1:] + ":" + prop + "}"
	}
	if k, ok := sy.keys[member]; ok {
		return "{config:" + k + "}"
	}
	if strings.ToUpper(member) == member && len(member) > 1 {
		return "{const:" + member + "}"
	}
	return "{var:" + member + "}"
}

func joinAddr(base, p string) string {
	if base == "" {
		return p
	}
	if p == "" {
		return base
	}
	if strings.Contains(p, "://") {
		return p
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(p, "/")
}

// splitTop splits an expression at a separator outside strings and
// brackets.
func splitTop(e, sep string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(e); i++ {
		switch e[i] {
		case '"', '\'', '`':
			i = skipString(e, i, langJS) - 1
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		default:
			if depth == 0 && strings.HasPrefix(e[i:], sep) {
				if sep == "." && (i+1 < len(e) && (e[i+1] >= '0' && e[i+1] <= '9' || e[i+1] == '.') || i > 0 && e[i-1] != ' ' && e[i+1] != ' ') {
					continue
				}
				out = append(out, strings.TrimSpace(e[start:i]))
				start = i + len(sep)
			}
		}
	}
	return append(out, strings.TrimSpace(e[start:]))
}

var (
	assignLine  = regexp.MustCompile(`^\s*(?:(?:private|public|protected|internal|readonly|static|final|const|let|var|val|export)\s+)*(?:[A-Z][\w<>.?]*\s+)?(?:self\s*\.\s*|this\s*\.\s*|\$this->|\$)?([A-Za-z_]\w*)\s*(?::\s*[\w<>?.|]+\s*)?(?::=|=)\s*(.+?)\s*;?\s*$`)
	objectField = regexp.MustCompile(`^\s*['"]?([A-Za-z_]\w*)['"]?\s*(?::|=>)\s*(.+?)\s*,?\s*$`)
)

// addresses records what fields and variables were assigned when it is an
// address, so a call made with the field renders the address it holds.
func (s *source) addresses(sy *symbols) {
	for line := 1; line <= len(s.lines); line++ {
		t := s.lineText(line)
		if !strings.ContainsAny(t, "=:") {
			continue
		}
		m := assignLine.FindStringSubmatch(t)
		if m == nil {
			m = objectField.FindStringSubmatch(t)
		}
		if m == nil || strings.HasPrefix(strings.TrimSpace(m[2]), "=") {
			continue
		}
		if _, done := sy.addrs[m[1]]; done {
			continue
		}
		r := s.renderAddr(m[2], sy)
		if strings.HasPrefix(r, "{env:") || strings.HasPrefix(r, "{config:") || strings.HasPrefix(r, "{prop:") || strings.Contains(r, "://") && !strings.ContainsAny(r, " \t") {
			sy.addrs[strings.Clone(m[1])] = strings.Clone(r)
		}
	}
}

// ruleHints are a literal each rule's pattern cannot match without, so a
// file that lacks it is never searched: the patterns that start at any
// identifier are what a scan pays for.
var ruleHints = func() []string {
	out := make([]string, len(outRules))
	for i, r := range outRules {
		re, err := syntax.Parse(r.re.String(), syntax.Perl)
		if err != nil {
			continue
		}
		out[i] = longest(required(re.Simplify()))
	}
	return out
}()

// required are literals a match must contain.
func required(re *syntax.Regexp) []string {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 {
			return nil
		}
		return []string{string(re.Rune)}
	case syntax.OpCapture, syntax.OpPlus:
		return required(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min > 0 {
			return required(re.Sub[0])
		}
	case syntax.OpConcat:
		var out []string
		for _, sub := range re.Sub {
			out = append(out, required(sub)...)
		}
		return out
	}
	return nil
}

func longest(list []string) string {
	best := ""
	for _, l := range list {
		if len(l) > len(best) {
			best = l
		}
	}
	if len(best) < 3 {
		return ""
	}
	return best
}

// callGateWords are the words a rule's receiver gate is made of, when it is
// nothing but words: a file without any of them has no receiver to gate.
var callGateWords = func() [][]string {
	out := make([][]string, len(callRules))
	for i, r := range callRules {
		g := strings.TrimPrefix(r.gate, "(?i)")
		if g == "" || strings.ContainsAny(g, `^$()[]\.*+?{}`) {
			continue
		}
		out[i] = strings.Split(strings.ToLower(g), "|")
	}
	return out
}()

func containsAny(text string, words []string) bool {
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}
