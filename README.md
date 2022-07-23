[![Go](https://github.com/RyanSusana/archstats/actions/workflows/ci.yml/badge.svg)](https://github.com/RyanSusana/archstats/actions/workflows/ci.yml)
# Archstats Introduction
Archstats is a command line tool that assists in
generating [package metrics for software projects](https://en.wikipedia.org/wiki/Software_package_metrics). It's based
on static code analysis.
It helps in answering questions like this:

- How many packages/components are there in the project?
- What are the afferent/efferent couplings between components/packages?
- How many functions/fields/classes/interfaces are there per component/file/directory?
- How many occurences of this _custom regex pattern_ are there per component/file/directory?
- _etc._ See more in the Examples section

# Installation
Archstats is distributed as a [Go module](https://go.dev/blog/using-go-modules). It can be installed like this:
```shell
go get -u github.com/RyanSusana/archstats
```

# Usage

For instructions on how to use Archstats and the available options, run:
```shell
archstats --help
```
Here's a simple example. It gets a count of all functions, per directory, in the project. **Notice the use of [named capture groups](https://www.regular-expressions.info/named.html)**:
```shell
archstats directories-recursive path/to/project --regex-snippet "function (?P<functions>.*)\(.*\)" --sorted-by functions
```

## Ignoring files
Archstats can be configured to ignore certain files. This is useful when there are files that you don't want to include in analysis.
Archstats recursively looks for `.gitignore`/`.archstatsignore` files throughout the project and ignores files & directories according to the [.gitignore format](https://git-scm.com/docs/gitignore).


# More Examples

### In my PHP project, I want to count how many statements there are in each component/namespace.

```shell
archstats components path/to/project --language php --regex-snippet "(?P<statements>.*;)" --sorted-by statements
```

### In my PHP project, I want to know how many functions are in each file.

```shell
archstats files path/to/project --language php --regex-snippet "function (?P<functions>.*)\(.*\)" --sorted-by functions
```

### In my PHP project, I want to see the connections (afferent/efferent couplings) between components.

```shell
archstats component-connections path/to/project --language php
```

### In my PHP project, I want to recursively count the number of Laravel routes per directory

```shell
archstats directories-recursive path/to/project --language php --regex-snippet "(?P<routes>Route::(.*))" --sorted-by routes
```

# FAQ
### 1. What is a component/package?

The term 'component' is loosely defined within the Software Industry. For the sake of alignment I chose to go with the
following definition by world-renowned
architect [Mark Richards](https://www.developertoarchitect.com/mark-richards.html):
> A component is the physical manifestation of a software module. They are the packages of your software system. There are the building blocks of your system.

For more information, see [this video](https://www.youtube.com/watch?v=jrohK2unyE8).

Here is a mapping between famous programming languages and their components:

| Language | Component                                                                                  |
| -------- |--------------------------------------------------------------------------------------------|
| C# | [Namespaces](https://docs.microsoft.com/en-us/dotnet/csharp/fundamentals/types/namespaces) |
| Java | [Packages](https://docs.oracle.com/javase/tutorial/java/concepts/package.html)             |
| JavaScript | [Modules](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Modules)           |
| Golang | [Packages](https://go.dev/tour/basics/1)                                                   |
| PHP | [Namespaces](https://www.php.net/manual/en/language.namespaces.php)                        |

### 2. What are snippets?

Snippets are the smallest units of code that can be analyzed in Archstats. They are references to the _significant_
contents of a file. These snippets are then aggregated to create insights for a codebase.
