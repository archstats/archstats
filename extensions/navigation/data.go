package navigation

import (
	"regexp"
	"strings"
)

// What the code stores and who touches it. A shared table is the coupling an
// import graph never shows: two modules that never import each other but both
// write `orders` cannot be split apart, and that is what sinks most
// extractions. An entity is read from how an ORM or a mapping file declares
// one; an access from a repository's type, an ORM call, or SQL in a string.

// An entity as declared: a class an ORM maps, or a table a mapping file or
// schema names.
type entity struct {
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

// An access is code using an entity or a table.
type access struct {
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
	accessRead      = "read"
	accessWrite     = "write"
	accessReadWrite = "read_write"
)

var (
	sqlStart      = regexp.MustCompile(`(?is)^\s*(select|insert|update|delete|with|merge|replace|upsert)\b`)
	sqlFrom       = regexp.MustCompile(`(?i)\b(from|join)\s+([A-Za-z_"` + "`" + `\[][\w."` + "`" + `\]]*)`)
	sqlInto       = regexp.MustCompile(`(?i)\b(insert\s+into|update|delete\s+from|merge\s+into|replace\s+into|upsert\s+into)\s+([A-Za-z_"` + "`" + `\[][\w."` + "`" + `\]]*)`)
	sqlSignals    = regexp.MustCompile(`(?i)jdbcTemplate|JdbcTemplate|createNativeQuery|createQuery|@Query|NamedParameterJdbc|executeQuery|executeUpdate|prepareStatement|cursor\.execute|\.execute\(|\.raw\(|knex|sequelize\.query|db\.query|\.Query\(|\.Exec\(|QueryRow|SqlCommand|Dapper|FromSqlRaw|ExecuteSqlRaw|DB::(select|statement|table)|createQueryBuilder|\$wpdb|pdo->|mysqli`)
	notTables     = map[string]bool{"select": true, "where": true, "dual": true, "the": true, "a": true, "an": true, "this": true, "that": true, "set": true, "values": true, "lateral": true, "unnest": true, "table": true, "only": true, "as": true, "on": true, "using": true, "json_table": true, "generate_series": true, "information_schema": true}
	repoGeneric   = regexp.MustCompile(`\b(?:Jpa|Crud|PagingAndSorting|ListCrud|ListPagingAndSorting|Mongo|ReactiveCrud|ReactiveMongo|R2dbc|Elasticsearch|Couchbase|Neo4j|Cassandra|Revision|QuerydslPredicateExecutor<|JpaSpecificationExecutor<)?Repository\s*<\s*([A-Z]\w*)`)
	iRepository   = regexp.MustCompile(`\bI?\w*Repository\s*<\s*([A-Z]\w*)\s*>`)
	getRepo       = regexp.MustCompile(`getRepository\s*\(\s*([A-Z]\w*)(?:::class|\.class)?\s*\)`)
	injectRepo    = regexp.MustCompile(`InjectRepository\s*\(\s*([A-Z]\w*)\s*\)`)
	djangoORM     = regexp.MustCompile(`\b([A-Z]\w*)\.(?:objects|_default_manager|all_objects)\.(\w+)`)
	eloquent      = regexp.MustCompile(`\b([A-Z]\w*)::(where\w*|find\w*|all|create|update\w*|destroy|query|first\w*|insert|upsert|firstOrCreate|updateOrCreate|with|select|count|paginate|get)\s*\(`)
	jsModelCall   = regexp.MustCompile(`\b([A-Z]\w*)\.(findOne|findAll|find|findById|findByPk|findOneAndUpdate|findByIdAndUpdate|findOneAndDelete|findByIdAndDelete|create|insertMany|updateOne|updateMany|deleteOne|deleteMany|destroy|update|upsert|aggregate|countDocuments|count|bulkCreate|exists)\s*\(`)
	efDbSet       = regexp.MustCompile(`\bDbSet\s*<\s*([A-Z]\w*)\s*>\s+([A-Z]\w*)`)
	efSet         = regexp.MustCompile(`\.Set\s*<\s*([A-Z]\w*)\s*>\s*\(\s*\)`)
	jpaFind       = regexp.MustCompile(`\.(find|getReference|persist|merge|remove)\s*\(\s*([A-Z]\w*)\.class`)
	tableName     = regexp.MustCompile(`__tablename__\s*=\s*["']([\w.]+)["']`)
	dbTable       = regexp.MustCompile(`\bdb_table\s*=\s*["']([\w.]+)["']`)
	phpTable      = regexp.MustCompile(`protected\s+\$table\s*=\s*["']([\w.]+)["']`)
	gormTable     = regexp.MustCompile(`func\s*\(\s*\w*\s*\*?([A-Z]\w*)\s*\)\s*TableName\s*\(\s*\)\s*string\s*\{\s*return\s+"([\w.]+)"`)
	prismaModel   = regexp.MustCompile(`(?m)^\s*model\s+([A-Za-z_]\w*)\s*\{`)
	prismaMap     = regexp.MustCompile(`@@map\(\s*"([\w.]+)"`)
	mongooseModel = regexp.MustCompile(`\b(?:mongoose\.)?model\s*(?:<[^>]*>)?\s*\(\s*["']([A-Za-z_]\w*)["']`)
	sequelizeDef  = regexp.MustCompile(`\.define\s*\(\s*["']([A-Za-z_]\w*)["']`)
	hbmClass      = regexp.MustCompile(`<(class|joined-subclass|union-subclass|subclass)\b[^>]*?\bname\s*=\s*"([\w.$]+)"[^>]*>`)
	doctrineXML   = regexp.MustCompile(`<(entity|mapped-superclass|document)\b[^>]*?\bname\s*=\s*"([\w\\]+)"[^>]*>`)
	xmlTable      = regexp.MustCompile(`\b(?:table|collection)\s*=\s*"([\w.]+)"`)
	djangoWrites  = map[string]bool{"create": true, "update": true, "delete": true, "bulk_create": true, "bulk_update": true, "get_or_create": true, "update_or_create": true, "save": true}
	jsWrites      = map[string]bool{"create": true, "insertMany": true, "updateOne": true, "updateMany": true, "deleteOne": true, "deleteMany": true, "destroy": true, "update": true, "upsert": true, "bulkCreate": true, "findOneAndUpdate": true, "findByIdAndUpdate": true, "findOneAndDelete": true, "findByIdAndDelete": true}
)

// entities reads what a file declares as stored.
func (s *source) entities() []entity {
	var out []entity
	switch s.lang {
	case langXML:
		for _, loc := range hbmClass.FindAllStringSubmatchIndex(s.code, -1) {
			if !strings.Contains(s.code, "hibernate-mapping") {
				break
			}
			tag := s.code[loc[0]:loc[1]]
			e := entity{Class: s.code[loc[4]:loc[5]], Store: "sql", Framework: "hibernate-xml", File: s.path, Line: s.line(loc[0])}
			if m := xmlTable.FindStringSubmatch(tag); m != nil {
				e.Table = m[1]
			}
			e.Name = e.Class[strings.LastIndexAny(e.Class, ".$")+1:]
			out = append(out, e)
		}
		for _, loc := range doctrineXML.FindAllStringSubmatchIndex(s.code, -1) {
			if !strings.Contains(s.code, "doctrine") {
				break
			}
			tag := s.code[loc[0]:loc[1]]
			e := entity{Class: s.code[loc[4]:loc[5]], Store: "sql", Framework: "doctrine-xml", File: s.path, Line: s.line(loc[0])}
			if s.code[loc[2]:loc[3]] == "document" {
				e.Store = "document"
			}
			if m := xmlTable.FindStringSubmatch(tag); m != nil {
				e.Table = m[1]
			}
			e.Name = e.Class[strings.LastIndexByte(e.Class, '\\')+1:]
			out = append(out, e)
		}
		return out
	case langPrisma:
		models := prismaModel.FindAllStringSubmatchIndex(s.code, -1)
		for i, loc := range models {
			end := len(s.code)
			if i+1 < len(models) {
				end = models[i+1][0]
			}
			e := entity{Name: s.code[loc[2]:loc[3]], Table: s.code[loc[2]:loc[3]], Store: "sql", Framework: "prisma", File: s.path, Line: s.line(loc[0])}
			if m := prismaMap.FindStringSubmatch(s.code[loc[1]:end]); m != nil {
				e.Table = m[1]
			}
			out = append(out, e)
		}
		return out
	}

	for _, d := range s.decls() {
		if !d.Class {
			continue
		}
		e := entity{Class: d.Name, Name: d.Name, File: s.path, Line: d.Line}
		switch {
		case d.has("Entity") != nil && (s.lang == langJava || s.lang == langKotlin || s.lang == langScala):
			e.Store, e.Framework = "sql", "jpa"
			e.Name = orDefault(d.has("Entity").arg("name"), d.Name)
		case d.has("Entity") != nil && s.lang == langJS:
			e.Store, e.Framework = "sql", "typeorm"
			e.Table = d.has("Entity").arg("name")
		case d.has("Entity") != nil && s.lang == langPHP:
			e.Store, e.Framework = "sql", "doctrine"
		case d.has("Document") != nil && (s.lang == langJava || s.lang == langKotlin || s.lang == langPHP):
			e.Store, e.Framework = "document", "spring-data"
			e.Table = d.has("Document").arg("collection", "value", "indexName")
		case d.has("Table") != nil && s.lang == langCSharp:
			e.Store, e.Framework = "sql", "ef"
		case d.has("Schema") != nil && s.lang == langJS && strings.Contains(s.raw, "@nestjs/mongoose"):
			e.Store, e.Framework = "document", "mongoose"
		default:
			continue
		}
		if t := d.has("Table"); t != nil && e.Table == "" {
			e.Table = t.arg("name", "value", "Name")
		}
		out = append(out, e)
	}

	types := s.typeDecls()
	classAt := func(line int) string {
		name := ""
		for _, t := range types {
			if t.Line <= line {
				name = t.Name
			}
		}
		return name
	}
	switch s.lang {
	case langPython:
		for _, m := range tableName.FindAllStringSubmatchIndex(s.code, -1) {
			line := s.line(m[0])
			out = append(out, entity{Class: classAt(line), Name: classAt(line), Table: s.code[m[2]:m[3]], Store: "sql", Framework: "sqlalchemy", File: s.path, Line: line})
		}
		for _, m := range dbTable.FindAllStringSubmatchIndex(s.code, -1) {
			line := s.line(m[0])
			// Meta is the class the option sits in; the model is the one
			// around it, the last class before Meta.
			name := ""
			for _, t := range types {
				if t.Line <= line && t.Name != "Meta" {
					name = t.Name
				}
			}
			out = append(out, entity{Class: name, Name: name, Table: s.code[m[2]:m[3]], Store: "sql", Framework: "django", File: s.path, Line: line})
		}
	case langPHP:
		for _, m := range phpTable.FindAllStringSubmatchIndex(s.code, -1) {
			line := s.line(m[0])
			out = append(out, entity{Class: classAt(line), Name: classAt(line), Table: s.code[m[2]:m[3]], Store: "sql", Framework: "eloquent", File: s.path, Line: line})
		}
	case langGo:
		for _, m := range gormTable.FindAllStringSubmatchIndex(s.code, -1) {
			out = append(out, entity{Class: s.code[m[2]:m[3]], Name: s.code[m[2]:m[3]], Table: s.code[m[4]:m[5]], Store: "sql", Framework: "gorm", File: s.path, Line: s.line(m[0])})
		}
	case langCSharp:
		for _, m := range efDbSet.FindAllStringSubmatchIndex(s.code, -1) {
			out = append(out, entity{Class: s.code[m[2]:m[3]], Name: s.code[m[2]:m[3]], Table: s.code[m[4]:m[5]], Store: "sql", Framework: "ef", File: s.path, Line: s.line(m[0])})
		}
	case langJS:
		for _, m := range mongooseModel.FindAllStringSubmatchIndex(s.code, -1) {
			if strings.Contains(s.raw, "mongoose") {
				out = append(out, entity{Name: s.code[m[2]:m[3]], Store: "document", Framework: "mongoose", File: s.path, Line: s.line(m[0])})
			}
		}
		for _, m := range sequelizeDef.FindAllStringSubmatchIndex(s.code, -1) {
			if strings.Contains(s.raw, "sequelize") {
				out = append(out, entity{Name: s.code[m[2]:m[3]], Table: s.code[m[2]:m[3]], Store: "sql", Framework: "sequelize", File: s.path, Line: s.line(m[0])})
			}
		}
	}
	return out
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// accesses reads what a file does with stored data.
func (s *source) accesses() []access {
	switch s.lang {
	case langJava, langKotlin, langScala, langCSharp, langGo, langJS, langPython, langPHP, langDart, langSwift:
	default:
		return nil
	}
	var out []access
	add := func(target, kind, via string, offset int) {
		if target == "" {
			return
		}
		out = append(out, access{Target: target, Access: kind, Via: via, File: s.path, Line: s.line(offset)})
	}
	for _, re := range []*regexp.Regexp{repoGeneric, iRepository} {
		for _, m := range re.FindAllStringSubmatchIndex(s.code, -1) {
			add(s.code[m[2]:m[3]], accessReadWrite, "repository", m[0])
		}
	}
	for _, re := range []*regexp.Regexp{getRepo, injectRepo} {
		for _, m := range re.FindAllStringSubmatchIndex(s.code, -1) {
			add(s.code[m[2]:m[3]], accessReadWrite, "repository", m[0])
		}
	}
	for _, m := range jpaFind.FindAllStringSubmatchIndex(s.code, -1) {
		kind := accessWrite
		if op := s.code[m[2]:m[3]]; op == "find" || op == "getReference" {
			kind = accessRead
		}
		add(s.code[m[4]:m[5]], kind, "orm", m[0])
	}
	switch s.lang {
	case langPython:
		for _, m := range djangoORM.FindAllStringSubmatchIndex(s.code, -1) {
			kind := accessRead
			if djangoWrites[s.code[m[4]:m[5]]] {
				kind = accessWrite
			}
			add(s.code[m[2]:m[3]], kind, "orm", m[0])
		}
	case langPHP:
		for _, m := range eloquent.FindAllStringSubmatchIndex(s.code, -1) {
			op := s.code[m[4]:m[5]]
			kind := accessRead
			if strings.HasPrefix(op, "update") || op == "create" || op == "destroy" || op == "insert" || op == "upsert" || op == "firstOrCreate" || op == "updateOrCreate" {
				kind = accessWrite
			}
			add(s.code[m[2]:m[3]], kind, "orm", m[0])
		}
	case langJS:
		for _, m := range jsModelCall.FindAllStringSubmatchIndex(s.code, -1) {
			kind := accessRead
			if jsWrites[s.code[m[4]:m[5]]] {
				kind = accessWrite
			}
			// Only a name that turns out to be an entity is kept; Promise.all
			// and Object.create are not tables.
			add(s.code[m[2]:m[3]], kind, "orm?", m[0])
		}
	case langCSharp:
		for _, m := range efSet.FindAllStringSubmatchIndex(s.code, -1) {
			add(s.code[m[2]:m[3]], accessReadWrite, "dbset", m[0])
		}
	}
	out = append(out, s.sqlAccesses()...)
	return out
}

// sqlAccesses are the tables named in SQL written as strings: the file's
// literals that are a statement, and in a file that runs SQL at all, every
// literal with a FROM, JOIN, INTO or UPDATE in it, since a statement built
// with `+` or a StringBuilder spreads its tables over several.
func (s *source) sqlAccesses() []access {
	lits := s.literals()
	runsSQL := sqlSignals.MatchString(s.code)
	if !runsSQL {
		for _, l := range lits {
			if sqlStart.MatchString(l.text) && (sqlFrom.MatchString(l.text) || sqlInto.MatchString(l.text)) {
				runsSQL = true
				break
			}
		}
	}
	if !runsSQL {
		return nil
	}
	var out []access
	seen := map[string]bool{}
	for _, l := range lits {
		if len(l.text) < 6 || !strings.ContainsAny(l.text, " \n\t") {
			continue
		}
		statement := sqlStart.MatchString(l.text)
		if !statement && !sqlFrom.MatchString(l.text) && !sqlInto.MatchString(l.text) {
			continue
		}
		if !statement && looksLikeProse(l.text) {
			continue
		}
		via := "sql"
		if s.lang == langJava || s.lang == langKotlin {
			// JPQL and HQL name entities with an alias right after:
			// `from Order o`. The linker decides by what the name resolves to.
			via = "sql|jpql"
		}
		line := s.line(l.offset)
		for _, m := range sqlInto.FindAllStringSubmatch(l.text, -1) {
			if t := tableIdent(m[2]); t != "" && !seen[t+"w"+itoa(line)] {
				seen[t+"w"+itoa(line)] = true
				out = append(out, access{Target: t, Access: accessWrite, Via: via, File: s.path, Line: line})
			}
		}
		for _, m := range sqlFrom.FindAllStringSubmatch(l.text, -1) {
			// DELETE FROM is a write and was read above.
			if strings.EqualFold(m[1], "from") && regexp.MustCompile(`(?i)delete\s*$`).MatchString(l.text[:strings.Index(l.text, m[0])]) {
				continue
			}
			if t := tableIdent(m[2]); t != "" && !seen[t+"r"+itoa(line)] {
				seen[t+"r"+itoa(line)] = true
				out = append(out, access{Target: t, Access: accessRead, Via: via, File: s.path, Line: line})
			}
		}
	}
	return out
}

// looksLikeProse is a sentence that happens to say "from the": an error
// message, a log line. SQL fragments are mostly identifiers and keywords.
func looksLikeProse(t string) bool {
	m := sqlFrom.FindStringSubmatch(t)
	if m == nil {
		m = sqlInto.FindStringSubmatch(t)
	}
	if m == nil {
		return true
	}
	name := strings.ToLower(tableIdent(m[2]))
	if name == "" || notTables[name] {
		return true
	}
	// "Failed to load data from server": a capitalised sentence with no SQL
	// punctuation and few underscores.
	words := strings.Fields(t)
	sqlish := strings.ContainsAny(t, "=?:(*,") || strings.Contains(t, "_") || strings.ToLower(t) != t && strings.ToUpper(t) == t
	return len(words) > 4 && !sqlish
}

// tableIdent is a table as SQL names it, quotes and brackets removed, a
// schema kept: "public"."orders" is public.orders.
func tableIdent(raw string) string {
	t := strings.NewReplacer(`"`, "", "`", "", "[", "", "]", "").Replace(raw)
	t = strings.Trim(t, ".")
	if t == "" || strings.HasPrefix(t, ":") || strings.HasPrefix(t, "?") || strings.HasPrefix(t, "$") || strings.ContainsAny(t, "(){}") {
		return ""
	}
	if notTables[strings.ToLower(t)] || len(t) < 2 {
		return ""
	}
	for _, r := range t {
		if !(r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return ""
		}
	}
	return t
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
