[![Go](https://github.com/RyanSusana/archstats/actions/workflows/ci.yml/badge.svg)](https://github.com/RyanSusana/archstats/actions/workflows/ci.yml)
# Archstats introduction
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
archstats path/to/project --regex-snippet "function (?P<functions>.*)\(.*\)" --sorted-by functions
```
## Components
The term 'component' is loosely defined within the software industry. For the sake of alignment I chose to go with the
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

## Snippets
Snippets are the smallest units of code that can be analyzed in Archstats. They are references to the _architecturally significant_
parts of a file. These snippets are then aggregated to create insights for a codebase.

## Views
Archstats supports a number of views. A view is an aggregation of the snippets found throughout the project. A view can be selected by using the `--view` or `-v` option.  The default view is `directories-recursive`.

| View                    | Description                                                                                                                                                                                                                                                                                                                                                                                                                          |
|-------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `snippets`              | A snippet is a reference to a piece of code that is _'architecturally significant'_ with respects to the project. A snippet can be something like a class declaration, an import statement, an if statement, a method declaration, a route definition, etc. It's up to you. Every snippet has a type, which is used to provide semantic meaning to the snippet. There are several [built-in snippet types](#built-in-snippet-types). |
| `directories-recursive` | Recursively goes through all the directories in a project and counts snippets by type                                                                                                                                                                                                                                                                                                                                                |
| `directories-flat`      | Goes through all the directories in a project and counts snippets by type                                                                                                                                                                                                                                                                                                                                                            |
| `files`                 | Goes through all the files in a project and counts snippets by type                                                                                                                                                                                                                                                                                                                                                                  |
| `components`            | Every unique snippet with a `componentDeclaration` type will generate a new component. If a file has a `componentDeclaration`, all snippets within that file will correspond to the related component. This view aggregates the counts of snippets per component. _Note_: requires `componentDeclaration` snippet type.                                                                                                              |
| `component-connections` | Every snippet with a type of `componentImport` that matches a `componentDeclaration` will create a 'component connection'. The connection has a 'from' as the `componentImport` and a 'to' as the `componentDeclaration`. _Note_: requires `componentDeclaration` and `componentImport` snippet types.                                                                                                                               |

## Built-in snippet types
Archstats has several built-in snippet types. These types are used to help provide semantic meaning to standard snippets across codebases.
| Type | Description                                                                                                                                                                                                                                                                                                                                                    |
|------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `componentDeclaration` | A component declaration is a snippet that defines a component within a file. It's usually something like a package/namespace/module declaration. More on components [here](#faq). An example of a java `componentDeclaration` is something like `package com.example.my.cool.package` where `com.example.my.cool.package` is the actual `componentDeclaration` |
| `componentImport`      | A component import is a snippet that defines the import of a component. It's usually an import/using statement in most languages. In java it looks like this `import com.example.my.cool.package.MyCoolClass` where `com.example.my.cool.package` is the actual `componentImport`  |                                                                              |
| `function`             | A function is a snippet that defines a function. It's usually a function declaration. In java it looks like this `public void myFunction()` where `myFunction` is the actual `function` |
| `abstractElement` | An abstract element is an interface or abstract class. In java it looks like this `public abstract class MyAbstractClass` where `MyAbstractClass` is the actual `abstractElement` |
| `class` | A class is a snippet that defines a class. It's usually a class declaration. In java it looks like this `public class MyClass` where `MyClass` is the actual `class` |


## Language extensions
Archstats supports a number of _optional_ language extensions. These extensions are used to assist users in getting started with Archstats. They pre-configure Archstats with build-in snippet types for the specified language.

Supported language extensions are:
- PHP

## Ignoring files
Archstats can be configured to ignore certain files. This is useful when there are files that you don't want to include in analysis.
Archstats recursively looks for `.gitignore`/`.archstatsignore` files throughout the project and ignores files & directories according to the [.gitignore format](https://git-scm.com/docs/gitignore).


# Examples

### In my PHP project, I want to count how many statements there are in each component/namespace.

```shell
archstats path/to/project --view components --language php --regex-snippet "(?P<statements>.*;)" --sorted-by statements
```

### In my PHP project, I want to know how many functions are in each file.

```shell
archstats path/to/project --view files --language php --regex-snippet "function (?P<functions>.*)\(.*\)" --sorted-by functions
```

### In my PHP project, I want to see the connections (afferent/efferent couplings) between components.

```shell
archstats path/to/project --view component-connections --language php
```

### In my PHP project, I want to recursively count the number of Laravel routes per directory

```shell
archstats path/to/project --view directories-recursive --language php --regex-snippet "(?P<routes>Route::(.*))" --sorted-by routes
```
