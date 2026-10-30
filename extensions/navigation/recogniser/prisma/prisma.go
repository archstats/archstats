// Package prisma reads Prisma schemas for navigation: the models they map
// to tables.
package prisma

import (
	"regexp"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

const prisma = "prisma"

var (
	prismaModel = regexp.MustCompile(`(?m)^\s*model\s+([A-Za-z_]\w*)\s*\{`)
	prismaMap   = regexp.MustCompile(`@@map\(\s*"([\w.]+)"`)
)

// Plugin is Prisma.
func Plugin() r.Plugin {
	return r.Plugin{
		Name: "prisma",
		Languages: []*r.Language{{
			Name: prisma, Extensions: []string{".prisma"}, Kind: r.Data,
			Syntax: r.Syntax{
				LineComments:  []string{"//"},
				BlockComments: [][2]string{{"/*", "*/"}},
				Strings:       true,
			},
		}},
		Recognisers: []*r.Recogniser{{
			Languages: []string{prisma},
			Entities:  entities,
		}},
	}
}

func entities(s *r.Source) []r.Entity {
	var out []r.Entity
	models := prismaModel.FindAllStringSubmatchIndex(s.Code, -1)
	for i, loc := range models {
		end := len(s.Code)
		if i+1 < len(models) {
			end = models[i+1][0]
		}
		e := r.Entity{Name: s.Code[loc[2]:loc[3]], Table: s.Code[loc[2]:loc[3]], Store: "sql", Framework: "prisma", File: s.Path, Line: s.Line(loc[0])}
		if m := prismaMap.FindStringSubmatch(s.Code[loc[1]:end]); m != nil {
			e.Table = m[1]
		}
		out = append(out, e)
	}
	return out
}
