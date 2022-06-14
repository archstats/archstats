package archstats

type Extension interface {
}
type AnalysisSettings struct {
	Extensions []Extension
}
type AnalysisResults struct {
	RootDirectory Directory
	Components    []Component
	Files         []File
	Directories   []Directory
}
type AfterFileProcessingResults struct {
	Files       []File
	Directories []Directory
}
type Summary struct {
	stats Stats
}

type PostFileProcessor interface {
	AfterFileProcessing(results *AfterFileProcessingResults)
}

func Analyze(rootPath string, settings AnalysisSettings) *AnalysisResults {
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
		Files:       root.FilesRecursive(),
		Directories: root.SubDirectoriesRecursive(),
	}
	for _, processor := range postProcessors {
		processor.AfterFileProcessing(afterFileProcessingResults)
	}

	results := &AnalysisResults{
		RootDirectory: root,
		Files:         root.FilesRecursive(),
		Directories:   root.SubDirectoriesRecursive(),
	}
	if componentGenerator == nil {
		results.Components = []Component{}
	} else {
		results.Components = componentGenerator.Components()
	}
	return results
}
