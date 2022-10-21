## Package Organization

| Package              | Description                                                                                                                 |
|:---------------------|:----------------------------------------------------------------------------------------------------------------------------|
| cmd                  | contains the CLI structure of the system                                                                                    |
| walker               | Responsible for walking a directory recursively, ignoring certain files/folders along the way                               |
| analysis             | Aggregates raw files into an easy to analyze `Results` struct. Also is the main way to hook into the analysis functionality |
| analysis/extensions  | Different extension analyzers                                                                                               |
| views                | Aggregates the information from `Results` into different `View`s                                                            |
| export               | Exports `View`s to various output formats                                                                                   |                                                                                                                     