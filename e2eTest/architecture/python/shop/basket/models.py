from oscar.core.loading import get_class, get_model

Product = get_model("catalogue", "Product")
Selector = get_class("catalogue.models", "Selector")
Missing = get_class("nowhere.at.all", "Thing")
Computed = get_class(some_variable, "Thing")

class Basket: pass
