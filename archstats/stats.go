package archstats

type Stats map[string]int

func (stats Stats) Merge(otherStats ...Stats) Stats {
	return MergeStats(append(otherStats, stats))
}

func MergeStats(maps []Stats) Stats {
	newStats := map[string]int{}

	for _, m := range maps {
		for k, v := range m {
			newStats[k] += v
		}
	}
	return newStats
}

type StatsView interface {
	Rows() []Measurable
}
type Measurable interface {
	Name() string
	Stats() Stats
}
