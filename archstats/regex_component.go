package archstats

import (
	"regexp"
)

func RegexBasedComponents(settings RegexBasedComponentSettings) *componentsExtension {
	return &componentsExtension{settings: settings,
		componentMap: map[string]*Component{},
	}
}

type componentsExtension struct {
	settings     RegexBasedComponentSettings
	connections  []*componentConnection
	componentMap map[string]*Component
}

type RegexBasedComponentSettings struct {
	Definition *regexp.Regexp
	Import     *regexp.Regexp
}

type componentConnection struct {
	from string
	to   string
}

func (c *componentsExtension) VisitFile(file *File, content []byte) {
	componentName := getComponentName(c.settings.Definition, content)
	if component, componentExists := c.componentMap[componentName]; componentExists {
		component.Files = append(component.Files, file)
	} else {
		c.componentMap[componentName] = &Component{
			Name:  componentName,
			Files: []*File{file},
		}
	}
	c.connections = append(c.connections, getConnections(c.settings.Import, componentName, content)...)
}

func (c *componentsExtension) Components() []*Component {
	linkConnectionsToComponents(c.connections, c.componentMap)
	components := make([]*Component, 0, len(c.componentMap))
	for _, component := range c.componentMap {
		component.Stats()
		component.AddStats(Stats{"files": len(component.Files)})
		component.AddStats(Stats{"efferent_coupling": len(component.OutgoingConnections)})
		component.AddStats(Stats{"afferent_coupling": len(component.IncomingConnections)})
		components = append(components, component)
	}
	return components
}

func linkConnectionsToComponents(connections []*componentConnection, componentMap map[string]*Component) {
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
