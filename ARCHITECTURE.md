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
| `component_declaration` | A component declaration is a snippet that defines a component within a file. It's usually something like a package/namespace/module declaration. More on components [here](#faq). An example of a java `componentDeclaration` is something like `package com.example.my.cool.package` where `com.example.my.cool.package` is the actual `componentDeclaration` |
| `component_import`      | A component import is a snippet that defines the import of a component. It's usually an import/using statement in most languages. In java it looks like this `import com.example.my.cool.package.MyCoolClass` where `com.example.my.cool.package` is the actual `componentImport`                                                                              |                                                                              |
| `function`              | A function is a snippet that defines a function. It's usually a function declaration. In java it looks like this `public void myFunction()` where `myFunction` is the actual `function`                                                                                                                                                                        |
| `abstract_type`         | An abstract type is an interface or abstract class. In java it looks like this `public abstract class MyAbstractClass` where `MyAbstractClass` is the actual `abstractElement`                                                                                                                                                                                 |
| `type`                  | A type is a snippet that defines a class. It's usually a class declaration. In java it looks like this `public class MyClass` where `MyClass` is the actual `class`                                                                                                                                                                                            |

Most of the built-in snippet types are used to support [metrics](https://en.wikipedia.org/wiki/Software_package_metrics)
such as coupling, abstractness and instability.