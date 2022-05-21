package archstats

type Extension interface {
}
type AnalysisSettings struct {
	Extensions []Extension
}
type AnalysisResults struct {
	RootDirectory *directory
	Components    []*component
	Files         []*file
	Directories   []*directory
}
type AfterFileProcessingResults struct {
	Files       []*file
	Directories []*directory
}
type Summary struct {
	stats Stats
}

type PostFileProcessor interface {
	AfterFileProcessing(results *AfterFileProcessingResults)
}

func Analyze(rootPath string, settings AnalysisSettings) AnalysisResults {
	var visitors []FileVisitor
	var postProcessors []PostFileProcessor
	var componentGenerator ComponentGenerator

	for _, extension := range settings.Extensions {
		if fv, isFileVisitor := extension.(FileVisitor); isFileVisitor {
			visitors = append(visitors, fv)
		}
		if pfp, isPostFileProcessor := extension.(PostFileProcessor); isPostFileProcessor {
			postProcessors = append(postProcessors, pfp)
		}
		if cg, isComponentGenerator := extension.(ComponentGenerator); isComponentGenerator {
			componentGenerator = cg
		}
	}
	root := processDirectory(rootPath, 0, visitors)

	afterFileProcessingResults := &AfterFileProcessingResults{
		Files:       root.GetDescendantFiles(),
		Directories: root.GetDescendantSubDirectories(),
	}
	for _, processor := range postProcessors {
		processor.AfterFileProcessing(afterFileProcessingResults)
	}

	results := AnalysisResults{
		RootDirectory: root,
		Components:    componentGenerator.Components(),
		Files:         root.GetDescendantFiles(),
		Directories:   root.GetDescendantSubDirectories(),
	}
	return results
}
