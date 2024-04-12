[![Go](https://github.com/archstats/archstats/actions/workflows/ci.yml/badge.svg)](https://github.com/archstats/archstats/actions/workflows/ci.yml)

# Archstats CLI

Archstats is a tool that assists in
generating insights for codebases, such
as [the traditional package metrics for software projects](https://en.wikipedia.org/wiki/Software_package_metrics).

**Archstats also has an open-source visualization tool called [Archstats UI](https://app.archstats.io).**


This is the CLI tool for Archstats. It is used to generate insights for codebases and provide DB files to be used
with [Archstats UI](https://app.archstats.io).

It helps in answering questions like this:

- How many packages/components are there in the project?
- What are the afferent/efferent couplings between components/packages?
- How many functions/fields/classes/interfaces are there per component/file/directory?
- How many occurences of this _custom regex pattern_ are there per component/file/directory?
- How many lines of code are there per component/file/directory?
- How tightly coupled are the components/packages from a code perspective, and from a historical perspective (git
  commits)?
- What are the most complex components/packages/files/directories?
- _etc._ See more in the Examples section


# Installation

Archstats is distributed as a [Go module](https://go.dev/blog/using-go-modules). It can be installed like this:

```shell
go install github.com/archstats/archstats@latest
```

Make sure that installed Go binaries are on your `PATH`. You can do so by running `go help install` and following the
instructions.

Archstats is also available as a binary for Windows, Linux and MacOS. You can download the latest release from the [releases page](https://github.com/archstats/archstats/releases).

## MacOS Installation
To install the latest release on MacOS, run the following command:
```shell
LATEST=$(curl -s https://api.github.com/repos/archstats/archstats/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
curl -L "https://github.com/archstats/archstats/releases/download/$LATEST/archstats-$LATEST-darwin-amd64.tar.gz" | sudo tar -xz -C /usr/local/bin
```

# Usage

For instructions on how to use Archstats and the available options, run:

```shell
archstats --help
```

Here's a simple example. It gets a count of all functions, per component, in the project.

```shell
archstats -f path/to/project view components --snippet "function (?P<functions>.*)\(.*\)" -e php -c name,abstractness,instability,functions,coupling:efferent:count --sorted-by abstractness
```

This might output something like this:

```shell
NAME                                                                                  ABSTRACTNESS          INSTABILITY            FUNCTIONS  EFFERENT_COUPLINGS
App\Mail\Base                                                                         1                     0.03571428571428571    0          1
App\Http\Controllers\Api\Business\v1                                                  1                     0.1674641148325359     17         35
App\Main\Repositories\Interfaces                                                      1                     0.07915831663326653    304        158
App\Main\Models\Collections\Base                                                      1                     0                      3          0
App\Http\Controllers                                                                  1                     0.11023622047244094    17         28
App\Main\Models\Interfaces                                                            1                     0.012319355602937692   3500       52
App\Http\Controllers\Api\Admin\v1                                                     1                     0.13333333333333333    2          4
App\Main\Automations\Actions                                                          1                     0.36                   11         9
App\Main\Algolia\Indeces                                                              1                     0.16666666666666666    8          1
App\Main\Models\Collections\Interfaces                                                1                     0.4666666666666667     11         7
... More Rows
```

## Components

The term 'component' is loosely defined within the software industry. For the sake of alignment I chose to go with the
following definition by world-renowned
architect [Mark Richards](https://www.developertoarchitect.com/mark-richards.html):
> A component is the physical manifestation of a software module. They are the packages of your software system. They
> are the building blocks of your system.

For more information, see [this video](https://www.youtube.com/watch?v=jrohK2unyE8).

## Extensions

Archstats supports a number of _optional_ extensions. These extensions are used to assist users in getting started with
Archstats. They pre-configure Archstats with built-in snippet types for specified languages and frameworks. They can be
configured with the `--extensions` or `-e` option.

Supported extensions are:

- `indentations` - Adds support for indentation based metrics, in order to measure complexity.
- `lines` - Adds support for line count based metrics, in order to measure number of lines in codebase per file or
  component.
- `php` - Adds support for PHP namespaces as components.
- `java` - Adds support for Java packages as components.
- `scala`- Adds support for Scala packages as components.
- `kotlin` - Adds support for Kotlin packages as components.
- `csharp` - Adds support for C# namespaces as components.
- `git` - Adds support for git log based views.
- `cycles` - Adds support for cycle detection views. _Be wary, this can be computationally expensive for large
  codebases. Avoid using this extension_

## Ignoring files

Archstats can be configured to ignore certain files. This is useful when there are files that you don't want to include
in analysis.
Archstats recursively looks for `.gitignore`/`.archstatsignore` files throughout the project and ignores files &
directories according to the [.gitignore format](https://git-scm.com/docs/gitignore).

# Examples

### In my PHP project, I want to count how many statements there are in each component/namespace.

```shell
archstats -f path/to/project view components --extension php --snippet "(?P<statements>.*;)" --sorted-by statements
```

### In my PHP project, I want to know how many functions are in each file.

```shell
archstats -f path/to/project view files --extension php --snippet "function (?P<functions>.*)\(.*\)" --sorted-by functions
```

### In my PHP project, I want to see the connections (afferent/efferent couplings) between components.

```shell
archstats -f path/to/project view component-connections --extension php
```

### In my PHP project, I want to recursively count the number of Laravel routes per directory

```shell
archstats -f path/to/project view directories-recursive --extension php --snippet "(?P<routes>Route::(.*))" --sorted-by routes
```
