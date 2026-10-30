package navigation

import (
	"regexp"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

// What the code stores and who touches it. A shared table is the coupling an
// import graph never shows: two modules that never import each other but both
// write `orders` cannot be split apart, and that is what sinks most
// extractions. An entity is read by a language's recognisers from how an ORM
// or a mapping file declares one; an access from a repository's type, an ORM
// call, or SQL in a string.

var (
	sqlStart    = regexp.MustCompile(`(?is)^\s*(select|insert|update|delete|with|merge|replace|upsert)\b`)
	sqlFrom     = regexp.MustCompile(`(?i)\b(from|join)\s+([A-Za-z_"` + "`" + `\[][\w."` + "`" + `\]]*)`)
	sqlInto     = regexp.MustCompile(`(?i)\b(insert\s+into|update|delete\s+from|merge\s+into|replace\s+into|upsert\s+into)\s+([A-Za-z_"` + "`" + `\[][\w."` + "`" + `\]]*)`)
	sqlSignals  = regexp.MustCompile(`(?i)jdbcTemplate|JdbcTemplate|createNativeQuery|createQuery|@Query|NamedParameterJdbc|executeQuery|executeUpdate|prepareStatement|cursor\.execute|\.execute\(|\.raw\(|knex|sequelize\.query|db\.query|\.Query\(|\.Exec\(|QueryRow|SqlCommand|Dapper|FromSqlRaw|ExecuteSqlRaw|DB::(select|statement|table)|createQueryBuilder|\$wpdb|pdo->|mysqli`)
	notTables   = map[string]bool{"select": true, "where": true, "dual": true, "the": true, "a": true, "an": true, "this": true, "that": true, "set": true, "values": true, "lateral": true, "unnest": true, "table": true, "only": true, "as": true, "on": true, "using": true, "json_table": true, "generate_series": true, "information_schema": true}
	repoGeneric = regexp.MustCompile(`\b(?:Jpa|Crud|PagingAndSorting|ListCrud|ListPagingAndSorting|Mongo|ReactiveCrud|ReactiveMongo|R2dbc|Elasticsearch|Couchbase|Neo4j|Cassandra|Revision|QuerydslPredicateExecutor<|JpaSpecificationExecutor<)?Repository\s*<\s*([A-Z]\w*)`)
	iRepository = regexp.MustCompile(`\bI?\w*Repository\s*<\s*([A-Z]\w*)\s*>`)
	getRepo     = regexp.MustCompile(`getRepository\s*\(\s*([A-Z]\w*)(?:::class|\.class)?\s*\)`)
	injectRepo  = regexp.MustCompile(`InjectRepository\s*\(\s*([A-Z]\w*)\s*\)`)
)

// entities reads what a file declares as stored.
func entities(s *source) []entity {
	var out []entity
	for _, rec := range s.Language().Recognisers() {
		if rec.Entities != nil {
			out = append(out, rec.Entities(s)...)
		}
	}
	return out
}

// accesses reads what a file does with stored data: a repository of a type,
// whatever the language spells it with; the ORM calls its recognisers read;
// and SQL in its strings.
func accesses(s *source) []access {
	if s.Language().Kind != r.Code {
		return nil
	}
	var out []access
	add := func(target, kind, via string, offset int) {
		if target == "" {
			return
		}
		out = append(out, access{Target: target, Access: kind, Via: via, File: s.Path, Line: s.Line(offset)})
	}
	for _, re := range []*regexp.Regexp{repoGeneric, iRepository} {
		for _, m := range re.FindAllStringSubmatchIndex(s.Code, -1) {
			add(s.Code[m[2]:m[3]], accessReadWrite, "repository", m[0])
		}
	}
	for _, re := range []*regexp.Regexp{getRepo, injectRepo} {
		for _, m := range re.FindAllStringSubmatchIndex(s.Code, -1) {
			add(s.Code[m[2]:m[3]], accessReadWrite, "repository", m[0])
		}
	}
	via := "sql"
	for _, rec := range s.Language().Recognisers() {
		if rec.Accesses != nil {
			for _, a := range rec.Accesses(s) {
				if a.Target != "" {
					out = append(out, a)
				}
			}
		}
		if rec.QueryVia != nil {
			if v := rec.QueryVia(s); v != "" {
				via = v
			}
		}
	}
	out = append(out, sqlAccesses(s, via)...)
	return out
}

// sqlAccesses are the tables named in SQL written as strings: the file's
// literals that are a statement, and in a file that runs SQL at all, every
// literal with a FROM, JOIN, INTO or UPDATE in it, since a statement built
// with `+` or a StringBuilder spreads its tables over several.
func sqlAccesses(s *source, via string) []access {
	lits := s.Literals()
	runsSQL := sqlSignals.MatchString(s.Code)
	if !runsSQL {
		for _, l := range lits {
			if sqlStart.MatchString(l.Text) && (sqlFrom.MatchString(l.Text) || sqlInto.MatchString(l.Text)) {
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
		if len(l.Text) < 6 || !strings.ContainsAny(l.Text, " \n\t") {
			continue
		}
		statement := sqlStart.MatchString(l.Text)
		if !statement && !sqlFrom.MatchString(l.Text) && !sqlInto.MatchString(l.Text) {
			continue
		}
		if !statement && looksLikeProse(l.Text) {
			continue
		}
		line := s.Line(l.Offset)
		for _, m := range sqlInto.FindAllStringSubmatch(l.Text, -1) {
			if t := tableIdent(m[2]); t != "" && !seen[t+"w"+itoa(line)] {
				seen[t+"w"+itoa(line)] = true
				out = append(out, access{Target: t, Access: accessWrite, Via: via, File: s.Path, Line: line})
			}
		}
		for _, m := range sqlFrom.FindAllStringSubmatch(l.Text, -1) {
			// DELETE FROM is a write and was read above.
			if strings.EqualFold(m[1], "from") && regexp.MustCompile(`(?i)delete\s*$`).MatchString(l.Text[:strings.Index(l.Text, m[0])]) {
				continue
			}
			if t := tableIdent(m[2]); t != "" && !seen[t+"r"+itoa(line)] {
				seen[t+"r"+itoa(line)] = true
				out = append(out, access{Target: t, Access: accessRead, Via: via, File: s.Path, Line: line})
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
