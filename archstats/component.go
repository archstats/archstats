package archstats

type Component interface {
	Measurable
	Files() []File
	Dependents() []Component
	Dependencies() []Component
}
type ComponentGenerator interface {
	Components() []Component
}
type component struct {
	name                string
	files               []File
	IncomingConnections []Component
	OutgoingConnections []Component
	stats               Stats
}

func (c *component) Files() []File {
	return c.files
}

func (c *component) Dependents() []Component {
	return c.IncomingConnections
}

func (c *component) Dependencies() []Component {
	return c.OutgoingConnections
}

func (c *component) AddStats(stats Stats) {
	c.stats = c.stats.Merge(stats)
}
func (c *component) Name() string {
	return c.name
}

func (c *component) Stats() Stats {
	if c.stats == nil {
		var allStats []Stats
		for _, file := range c.files {
			allStats = append(allStats, file.Stats())
		}
		c.stats = MergeStats(allStats)
	}
	return c.stats
}
