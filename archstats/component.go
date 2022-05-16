package archstats

type ComponentGenerator interface {
	Components() []*Component
}
type Component struct {
	Name                string
	Files               []*File
	IncomingConnections []*Component
	OutgoingConnections []*Component
	stats               Stats
}

func (c *Component) AddStats(stats Stats) {
	c.stats = c.stats.Merge(stats)
}
func (c *Component) Identity() string {
	return c.Name
}

func (c *Component) Stats() Stats {
	if c.stats == nil {
		var allStats []Stats
		for _, file := range c.Files {
			allStats = append(allStats, file.Stats())
		}
		c.stats = MergeStats(allStats)
	}
	return c.stats
}
