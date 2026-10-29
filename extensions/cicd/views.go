package cicd

import "github.com/archstats/archstats/core"

// The snapshot tables. Every file column is written the walker's way, so it
// joins files.name.

func (e *extension) pipelinesView(*core.Results) *core.View {
	var rows []*core.Row
	for _, p := range e.m().Pipelines {
		rows = append(rows, &core.Row{Data: core.RowData{
			"id": walkerName(p.ID), "name": p.Name, "system": p.System, "kind": p.Kind, "repository": p.Repository,
			"file": walkerName(p.File), "line": p.Line, "read": p.Read, "jobs": p.Jobs, "steps": p.Steps, "stages": p.Stages,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("id"), core.StringColumn("name"), core.StringColumn("system"), core.StringColumn("kind"),
		core.StringColumn("repository"), core.StringColumn("file"), core.IntColumn("line"), core.StringColumn("read"),
		core.IntColumn("jobs"), core.IntColumn("steps"), core.StringColumn("stages"),
	}, rows)
}

func (e *extension) unreadView(*core.Results) *core.View {
	var rows []*core.Row
	for _, u := range e.m().Unread {
		rows = append(rows, &core.Row{Data: core.RowData{
			"pipeline": walkerName(u.Pipeline), "from_line": u.FromLine, "to_line": u.ToLine, "reason": u.Reason, "file": walkerName(u.File),
		}})
	}
	return view([]*core.Column{
		core.StringColumn("pipeline"), core.IntColumn("from_line"), core.IntColumn("to_line"), core.StringColumn("reason"), core.StringColumn("file"),
	}, rows)
}

func (e *extension) triggersView(*core.Results) *core.View {
	var rows []*core.Row
	for _, t := range e.m().Triggers {
		rows = append(rows, &core.Row{Data: core.RowData{
			"pipeline": walkerName(t.Pipeline), "event": t.Event, "filter": t.Filter, "file": walkerName(t.File), "line": t.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("pipeline"), core.StringColumn("event"), core.StringColumn("filter"), core.StringColumn("file"), core.IntColumn("line"),
	}, rows)
}

func (e *extension) jobsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, j := range e.m().Jobs {
		rows = append(rows, &core.Row{Data: core.RowData{
			"pipeline": walkerName(j.Pipeline), "id": j.ID, "name": j.Name, "group": j.Group, "depth": j.Depth, "needs": j.Needs,
			"runner": j.Runner, "matrix": j.Matrix, "environment": j.Environment, "gate": j.Gate, "condition": j.Condition,
			"file": walkerName(j.File), "line": j.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("pipeline"), core.StringColumn("id"), core.StringColumn("name"), core.StringColumn("group"),
		core.IntColumn("depth"), core.StringColumn("needs"), core.StringColumn("runner"), core.IntColumn("matrix"),
		core.StringColumn("environment"), core.StringColumn("gate"), core.StringColumn("condition"),
		core.StringColumn("file"), core.IntColumn("line"),
	}, rows)
}

func (e *extension) stepsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, s := range e.m().Steps {
		rows = append(rows, &core.Row{Data: core.RowData{
			"pipeline": walkerName(s.Pipeline), "job": s.Job, "index": s.Index, "name": s.Name, "verb": s.Verb,
			"tool": s.Tool, "target": s.Target, "text": s.Text, "file": walkerName(s.File), "line": s.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("pipeline"), core.StringColumn("job"), core.IntColumn("index"), core.StringColumn("name"),
		core.StringColumn("verb"), core.StringColumn("tool"), core.StringColumn("target"), core.StringColumn("text"),
		core.StringColumn("file"), core.IntColumn("line"),
	}, rows)
}

func (e *extension) handoffsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, h := range e.m().Handoffs {
		rows = append(rows, &core.Row{Data: core.RowData{
			"pipeline": walkerName(h.Pipeline), "job": h.Job, "kind": h.Kind, "target": h.Target, "ref": h.Ref,
			"pinned": h.Pinned, "via": h.Via, "local_pipeline": walkerName(h.LocalPipeline), "file": walkerName(h.File), "line": h.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("pipeline"), core.StringColumn("job"), core.StringColumn("kind"), core.StringColumn("target"),
		core.StringColumn("ref"), core.StringColumn("pinned"), core.StringColumn("via"), core.StringColumn("local_pipeline"),
		core.StringColumn("file"), core.IntColumn("line"),
	}, rows)
}

func (e *extension) outputsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, o := range e.m().Outputs {
		rows = append(rows, &core.Row{Data: core.RowData{
			"pipeline": walkerName(o.Pipeline), "job": o.Job, "step": o.Step, "kind": o.Kind, "platform": o.Platform,
			"environment": o.Environment, "target": o.Target, "artifact": o.Artifact, "server": o.Server, "context": o.Context,
			"file": walkerName(o.File), "line": o.Line, "resolution": o.Resolution,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("pipeline"), core.StringColumn("job"), core.IntColumn("step"), core.StringColumn("kind"),
		core.StringColumn("platform"), core.StringColumn("environment"), core.StringColumn("target"), core.StringColumn("artifact"),
		core.StringColumn("server"), core.StringColumn("context"), core.StringColumn("file"), core.IntColumn("line"),
		core.StringColumn("resolution"),
	}, rows)
}

func (e *extension) artifactsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, d := range e.m().Artifacts {
		rows = append(rows, &core.Row{Data: core.RowData{
			"id": d.ID, "name": d.Name, "kind": d.Kind, "platform": d.Platform, "repository": d.Repository,
			"dockerfile": walkerName(d.Dockerfile), "context": d.Context, "module": d.Module, "built_by": d.BuiltBy,
			"base": d.BaseImage, "runtime": d.Runtime, "files": d.Files, "components": d.Components,
			"file": walkerName(d.File), "line": d.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("id"), core.StringColumn("name"), core.StringColumn("kind"), core.StringColumn("platform"),
		core.StringColumn("repository"), core.StringColumn("dockerfile"), core.StringColumn("context"), core.StringColumn("module"),
		core.StringColumn("built_by"), core.StringColumn("base"), core.StringColumn("runtime"), core.IntColumn("files"),
		core.IntColumn("components"), core.StringColumn("file"), core.IntColumn("line"),
	}, rows)
}

func (e *extension) contentsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, c := range e.m().Contents {
		rows = append(rows, &core.Row{Data: core.RowData{
			"artifact": c.Artifact, "path": c.Path, "pattern": c.Pattern, "module": c.Module,
			"file": walkerName(c.File), "line": c.Line, "resolution": c.Resolution,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("artifact"), core.StringColumn("path"), core.StringColumn("pattern"), core.StringColumn("module"),
		core.StringColumn("file"), core.IntColumn("line"), core.StringColumn("resolution"),
	}, rows)
}

func (e *extension) componentsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, c := range e.m().Components {
		rows = append(rows, &core.Row{Data: core.RowData{"artifact": c.Artifact, "component": c.Component, "files": c.Files}})
	}
	return view([]*core.Column{core.StringColumn("artifact"), core.StringColumn("component"), core.IntColumn("files")}, rows)
}

func (e *extension) dockerfilesView(*core.Results) *core.View {
	var rows []*core.Row
	for _, d := range e.m().Dockerfiles {
		rows = append(rows, &core.Row{Data: core.RowData{
			"file": walkerName(d.File), "repository": d.Repository, "stages": d.Stages, "final_base": d.FinalBase,
			"final_user": d.FinalUser, "ports": d.Ports, "entrypoint": d.Entrypoint, "healthcheck": d.Healthcheck,
			"dockerignore": d.Dockerignore,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("file"), core.StringColumn("repository"), core.IntColumn("stages"), core.StringColumn("final_base"),
		core.StringColumn("final_user"), core.StringColumn("ports"), core.StringColumn("entrypoint"),
		core.IntColumn("healthcheck"), core.IntColumn("dockerignore"),
	}, rows)
}

func (e *extension) stagesView(*core.Results) *core.View {
	var rows []*core.Row
	for _, s := range e.m().Stages {
		rows = append(rows, &core.Row{Data: core.RowData{
			"file": walkerName(s.File), "index": s.Index, "name": s.Name, "from": s.From, "from_kind": s.FromKind,
			"image": s.Image, "tag": s.Tag, "digest": s.Digest, "user": s.User, "line": s.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("file"), core.IntColumn("index"), core.StringColumn("name"), core.StringColumn("from"),
		core.StringColumn("from_kind"), core.StringColumn("image"), core.StringColumn("tag"), core.StringColumn("digest"),
		core.StringColumn("user"), core.IntColumn("line"),
	}, rows)
}

func (e *extension) flagsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, f := range e.m().Flags {
		subject := f.Subject
		if f.SubjectKind == "dockerfile" || f.SubjectKind == "pipeline" {
			subject = walkerName(subject)
		}
		rows = append(rows, &core.Row{Data: core.RowData{
			"rule": f.Rule, "subject_kind": f.SubjectKind, "subject": subject, "detail": f.Detail,
			"file": walkerName(f.File), "line": f.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("rule"), core.StringColumn("subject_kind"), core.StringColumn("subject"), core.StringColumn("detail"),
		core.StringColumn("file"), core.IntColumn("line"),
	}, rows)
}

func (e *extension) unresolvedView(*core.Results) *core.View {
	var rows []*core.Row
	for _, u := range e.m().Unresolved {
		from := u.From
		// A workload is keyed file#name internally; the name is what a
		// reader recognises.
		if i := lastIndexByte(from, '#'); i >= 0 {
			from = from[i+1:]
		} else {
			from = walkerName(from)
		}
		rows = append(rows, &core.Row{Data: core.RowData{
			"from": from, "ref": u.Ref, "file": walkerName(u.File), "line": u.Line, "reason": u.Reason,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("from"), core.StringColumn("ref"), core.StringColumn("file"), core.IntColumn("line"), core.StringColumn("reason"),
	}, rows)
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
