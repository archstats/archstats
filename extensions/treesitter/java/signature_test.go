package java

import "testing"

// A signature is the declaration a reader would scan for, without the
// annotations stacked on it.
func TestJavaSignature(t *testing.T) {
	_, fns := measureJava(`package p;
@RestController
class OrderController {
  @Override @Deprecated public String toString() { return ""; }

  @PostMapping("/orders")
  public ResponseEntity<Order> place(
      @RequestBody Cart cart,
      Customer customer) throws IOException {
    return null;
  }
}
`)
	for name, want := range map[string]string{
		"OrderController.toString": "public String toString()",
		"OrderController.place":    "public ResponseEntity<Order> place( @RequestBody Cart cart, Customer customer) throws IOException",
	} {
		if got := fns[name].Signature; got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}
