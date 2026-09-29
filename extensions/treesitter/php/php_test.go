package php

import (
	"io/fs"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const order = `<?php

declare(strict_types=1);

namespace Acme\Order\Model;

use Acme\Core\Entity\BaseEntity;
use Acme\Core\{Clock, Money as Cash};
use Acme\Payment;
use function Acme\Util\helper;
use Doctrine\ORM\Mapping as ORM;

/**
 * @ORM\Entity(repositoryClass="OrderRepository")
 */
#[ORM\Table(name: 'orders')]
abstract class Order extends BaseEntity implements OrderInterface, \Countable
{
    use TimestampableTrait;

    private ?Customer $customer = null;

    public function __construct(private Clock $clock, Cash $total) {}

    public function items(): ItemCollection
    {
        return new ItemCollection(Item::create(), \Acme\Other\Thing::FOO, Payment\Method::class);
    }

    public static function make(): static
    {
        if ($x instanceof Refund) {}
        try {} catch (OrderException $e) {}
        helper();
        return new self();
    }
}

interface OrderInterface extends \Stringable {}
trait TimestampableTrait {}
enum Status: string { case New = 'new'; }
function topLevel(Order $o): void {}
`

func analyse(t *testing.T, path, src string) *file.Results {
	t.Helper()
	res := createPHPLanguagePack().AnalyzeFileContent(path, []byte(src))
	require.NotNil(t, res)
	res.Units = unitsFrom(path, res)
	return res
}

func byID(units []*unit.Unit) map[string]*unit.Unit {
	out := map[string]*unit.Unit{}
	for _, u := range units {
		out[u.ID] = u
	}
	return out
}

func TestTypesAndFunctionsAreUnitsNamedByNamespace(t *testing.T) {
	units := byID(analyse(t, "src/Order/Model/Order.php", order).Units)
	for _, id := range []string{`Acme\Order\Model\Order`, `Acme\Order\Model\OrderInterface`, `Acme\Order\Model\TimestampableTrait`, `Acme\Order\Model\Status`} {
		require.Containsf(t, units, id, "%s is missing", id)
		assert.Equal(t, unit.KindType, units[id].Kind)
	}
	require.Contains(t, units, `Acme\Order\Model\topLevel`)
	assert.Equal(t, unit.KindFunction, units[`Acme\Order\Model\topLevel`].Kind)
}

func TestSupertypesAttributesAndDocblockAnnotationsAreMarkers(t *testing.T) {
	o := byID(analyse(t, "Order.php", order).Units)[`Acme\Order\Model\Order`]
	require.NotNil(t, o)
	assert.True(t, o.HasMarker("BaseEntity"))
	assert.True(t, o.HasMarker("OrderInterface"))
	assert.True(t, o.HasMarker("Countable"))
	assert.True(t, o.HasMarker("Table"), "an attribute")
	assert.True(t, o.HasMarker("Entity"), "a Doctrine docblock annotation")
}

// PHP resolves a class name through a use alias, else in the file's own
// namespace; a leading backslash is already fully qualified.
func TestReferencesResolveTheWayPHPDoes(t *testing.T) {
	o := byID(analyse(t, "Order.php", order).Units)[`Acme\Order\Model\Order`]
	require.NotNil(t, o)
	want := []unit.Ref{
		{Module: `Acme\Core\Entity`, Name: "BaseEntity", Exact: true},
		{Module: `Acme\Order\Model`, Name: "OrderInterface", Exact: true},
		{Module: `Acme\Order\Model`, Name: "TimestampableTrait", Exact: true},
		{Module: `Acme\Order\Model`, Name: "Customer", Exact: true},
		{Module: `Acme\Core`, Name: "Clock", Exact: true},
		{Module: `Acme\Core`, Name: "Money", Exact: true},
		{Module: `Acme\Order\Model`, Name: "ItemCollection", Exact: true},
		{Module: `Acme\Order\Model`, Name: "Item", Exact: true},
		{Module: `Acme\Other`, Name: "Thing", Exact: true},
		{Module: `Acme\Payment`, Name: "Method", Exact: true},
		{Module: `Acme\Order\Model`, Name: "Refund", Exact: true},
		{Module: `Acme\Order\Model`, Name: "OrderException", Exact: true},
		{Module: `Acme\Util`, Name: "helper", Exact: true},
		{Module: `Doctrine\ORM\Mapping`, Name: "Table", Exact: true},
	}
	for _, r := range want {
		assert.Containsf(t, o.Refs, r, "%s\\%s", r.Module, r.Name)
	}
	for _, r := range o.Refs {
		assert.NotEqual(t, "self", r.Name)
		assert.NotEqual(t, "static", r.Name)
	}
}

func TestImportsNameTheNamespaceTheyDependOn(t *testing.T) {
	res := analyse(t, "Order.php", order)
	var imports, raw []string
	for _, s := range res.Snippets {
		switch s.Type {
		case file.ComponentImport:
			imports = append(imports, s.Value)
		case file.ImportRaw:
			raw = append(raw, s.Value)
		}
	}
	assert.ElementsMatch(t, []string{`Acme\Core\Entity`, `Acme\Core`, `Acme`, `Acme\Util`, `Doctrine\ORM`}, imports)
	assert.Contains(t, raw, `Doctrine\ORM\Mapping`)
	assert.Contains(t, raw, `Acme\Core`)
	assert.Equal(t, `Acme\Order\Model`, res.Component)
}

func TestTypeCounts(t *testing.T) {
	res := analyse(t, "Order.php", order)
	counts := map[string]int{}
	for _, st := range res.Stats {
		if v, ok := st.Value.(int); ok {
			counts[st.StatType] += v
		}
	}
	assert.Equal(t, 4, counts["modularity__types__total"])
	assert.Equal(t, 2, counts["modularity__types__abstract"], "the abstract class and the interface")
}

// A routes or config file declares nothing and references plenty; those
// references belong to the file.
func TestFileLevelReferencesBelongToTheFile(t *testing.T) {
	src := `<?php
use App\Http\Controllers\OrderController;
use Illuminate\Support\Facades\Route;

Route::get('/orders', [OrderController::class, 'index']);
`
	units := byID(analyse(t, "routes/web.php", src).Units)
	mod := units["routes/web#"]
	require.NotNil(t, mod)
	assert.Equal(t, unit.KindModule, mod.Kind)
	assert.Contains(t, mod.Refs, unit.Ref{Module: `App\Http\Controllers`, Name: "OrderController", Exact: true})
	assert.Contains(t, mod.Refs, unit.Ref{Module: `Illuminate\Support\Facades`, Name: "Route", Exact: true})
}

// A PHP file is often a template, with PHP in islands between HTML.
func TestTemplatesAreRead(t *testing.T) {
	src := "<html><body>\n<?php use App\\View\\Helper; ?>\n<p><?= Helper::title() ?></p>\n</body></html>\n"
	units := byID(analyse(t, "templates/page.php", src).Units)
	mod := units["templates/page#"]
	require.NotNil(t, mod)
	assert.Contains(t, mod.Refs, unit.Ref{Module: `App\View`, Name: "Helper", Exact: true})
}

// Helper captures build units and are not stored: every name in every
// expression is one.
func TestHelperCapturesAreNotKept(t *testing.T) {
	res := (&phpAnalyzer{lp: createPHPLanguagePack()}).AnalyzeFile(&source{path: "Order.php", content: []byte(order)})
	require.NotNil(t, res)
	assert.NotEmpty(t, res.Units)
	for _, s := range res.Snippets {
		switch s.Type {
		case captureSpan, captureDoc, captureUse, captureCall, captureImportDecl, captureInterface:
			t.Fatalf("%s was stored", s.Type)
		}
	}
}

type source struct {
	fs.FileInfo
	path    string
	content []byte
}

func (s *source) Path() string    { return s.path }
func (s *source) Content() []byte { return s.content }
