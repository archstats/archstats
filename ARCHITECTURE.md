# Architectural Characteristics

To accompany Archstats functional goal of creating actionable insights for codebases, the following architectural
characteristics are chosen to drive decision making:

- Extensibility - The ability to extend the system with new functionality. To support this characteristic, Archstats is
  designed to be modular and pluggable, through a microkernel style architecture.
- Interoperability - The ability to work with other systems. To support this characteristic, Archstats CLI & Archstats UI is designed to
  support a variety of input and output formats, such as: JSON, CSV, SQLite, etc.
- User-friendliness - Archstats is designed to make software architecture as approachable as possible. To support this
  characteristic, Archstats is designed to be easy & pleasant to use.


# Domain Model

## Views

Views are the primary data outputs of Archstats. They are the results of analyzing a codebase.

Views consist of Columns and Rows, which are used to display data in a tabular format.

## Definitions

Every column in a view has a definition. A definition is used to provide semantic meaning to the column. Definitions are
used by Archstats to provide a consistent user-friendly experience across Archstats CLI & Archstats UI.

## Snippets

Snippets are the smallest units of code that can be analyzed in Archstats. They are references to the _architecturally
significant_
parts of a file. These snippets are then aggregated to create insights for a codebase.

Every snippet has a type, which is used to provide semantic meaning to the snippet. Snippet types are normalized to be
lowecase.

## Built-in snippet types

Archstats has several built-in snippet types. These types are used to help provide semantic meaning to standard snippets
across codebases.

| Type                    | Description                                                                                                                                                                                                                                                                                                                                                    |
|-------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `component:declaration` | A component declaration is a snippet that defines a component within a file. It's usually something like a package/namespace/module declaration. More on components [here](#faq). An example of a java `componentDeclaration` is something like `package com.example.my.cool.package` where `com.example.my.cool.package` is the actual `componentDeclaration` |
| `component:import`      | A component import is a snippet that defines the import of a component. It's usually an import/using statement in most languages. In java it looks like this `import com.example.my.cool.package.MyCoolClass` where `com.example.my.cool.package` is the actual `componentImport`                                                                              |                                                                              |
| `function`              | A function is a snippet that defines a function. It's usually a function declaration. In java it looks like this `public void myFunction()` where `myFunction` is the actual `function`                                                                                                                                                                        |
| `type:abstract`         | An abstract type is an interface or abstract class. In java it looks like this `public abstract class MyAbstractClass` where `MyAbstractClass` is the actual `abstractElement`                                                                                                                                                                                 |
| `type`                  | A type is a snippet that defines a class. It's usually a class declaration. In java it looks like this `public class MyClass` where `MyClass` is the actual `class`                                                                                                                                                                                            |

Most of the built-in snippet types are used to support [metrics](https://en.wikipedia.org/wiki/Software_package_metrics)
such as coupling, abstractness and instability.