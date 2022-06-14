package archstats

import (
	"regexp"
)

func RegexBasedComponents(settings RegexBasedComponentSettings) *componentsExtension {
	return &componentsExtension{settings: settings,
		componentMap: map[string]*component{},
	}
}

type componentsExtension struct {
	settings     RegexBasedComponentSettings
	connections  []*componentConnection
	componentMap map[string]*component
}

type RegexBasedComponentSettings struct {
	Definition *regexp.Regexp
	Import     *regexp.Regexp
}

type componentConnection struct {
	from string
	to   string
}

func (c *componentsExtension) VisitFile(file File, content []byte) {
	componentName := getComponentName(c.settings.Definition, content)
	if len(componentName) == 0 {
		componentName = "N/A"
	}
	if comp, componentExists := c.componentMap[componentName]; componentExists {
		comp.files = append(comp.files, file)
	} else {
		c.componentMap[componentName] = &component{
			name:  componentName,
			files: []File{file},
		}
	}
	c.connections = append(c.connections, getConnections(c.settings.Import, componentName, content)...)
}

func (c *componentsExtension) Components() []Component {
	linkConnectionsToComponents(c.connections, c.componentMap)
	components := make([]Component, 0, len(c.componentMap))
	for _, comp := range c.componentMap {
		comp.Stats()
		comp.AddStats(Stats{"files": len(comp.files)})
		comp.AddStats(Stats{"efferent_coupling": len(comp.OutgoingConnections)})
		comp.AddStats(Stats{"afferent_coupling": len(comp.IncomingConnections)})
		components = append(components, comp)
	}
	return components
}

func linkConnectionsToComponents(connections []*componentConnection, componentMap map[string]*component) {
	for _, connection := range connections {
		from, hasFromConnection := componentMap[connection.from]
		to, hasToConnection := componentMap[connection.to]

		if hasToConnection && hasFromConnection {
			from.OutgoingConnections = append(from.OutgoingConnections, to)
			to.IncomingConnections = append(to.IncomingConnections, from)
		}
	}
}
func getConnections(regex *regexp.Regexp, fromComponentName string, fileContent []byte) []*componentConnection {
	var connections []*componentConnection
	matches := regex.FindAllSubmatch(fileContent, -1)

	for _, match := range matches {
		result := make(map[string]string)
		for i, name := range regex.SubexpNames() {
			if i != 0 && name != "" && len(match) > i {
				result[name] = string(match[i])
			}
		}
		connections = append(connections, &componentConnection{
			from: fromComponentName,
			to:   result["component"],
		})
	}
	return connections
}

func getComponentName(regex *regexp.Regexp, fileContent []byte) string {
	match := regex.FindSubmatch(fileContent)

	names := regex.SubexpNames()
	result := make(map[string]string)
	for i, name := range names {
		if i != 0 && name != "" && len(match) > i {
			result[name] = string(match[i])
		}
	}
	return result["component"]
}
