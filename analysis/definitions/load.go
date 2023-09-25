package definitions

//
//func LoadYamlFiles(fs embed.FS) ([]*Definition, error) {
//
//	// scan dir for yaml files recursively
//
//	yamlFiles, err := getYamlFiles(".", fs)
//	if err != nil {
//		return nil, err
//	}
//
//
//
//	wg := &sync.WaitGroup{}
//	wg.Add(len(yamlFiles))
//	for _, theFile := range yamlFiles {
//		go func(file , group *sync.WaitGroup) {
//
//			//TODO cleanup error handling
//			content, _ := os.ReadFile(file.Path())
//			openedFile := &absoluteFile{
//				path:    file.Path(),
//				info:    file.Info(),
//				content: content,
//			}
//
//			visitor(openedFile)
//			group.Done()
//		}(theFile, wg)
//	}
//	wg.Wait()
//
//}
//
//func getYamlFiles(currentDirectory string, fileSystem fs.ReadDirFS) ([]fs.DirEntry, error) {
//	var files []fs.DirEntry
//
//	dir, err := fileSystem.ReadDir(currentDirectory)
//	if err != nil {
//		return nil, err
//	}
//
//	for _, entry := range dir {
//
//		if entry.IsDir() {
//			yamlFiles, err := getYamlFiles(currentDirectory+"/"+entry.Name(), fs)
//			if err != nil {
//				return nil, err
//			}
//			files = append(files, yamlFiles...)
//		}
//
//		if strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml") {
//			files = append(files, currentDirectory+"/"+entry.Name())
//		}
//	}
//	return files, nil
//}
