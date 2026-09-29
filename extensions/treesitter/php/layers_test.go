package php

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Laravel app and a Symfony app, read the way the lanes read them. The
// UI's layers test (archstats-ui, layers.server.test.ts) is built from what
// these assert: the markers with their source, the raw imports, and the refs
// that become edges between layers.

func super(key string) unit.Marker     { return unit.Marker{Source: unit.SourceSupertype, Key: key} }
func attribute(key string) unit.Marker { return unit.Marker{Source: unit.SourceAnnotation, Key: key} }

func rawImports(res *file.Results) []string {
	var out []string
	for _, s := range res.Snippets {
		if s.Type == file.ImportRaw {
			out = append(out, s.Value)
		}
	}
	return out
}

func TestLaravelControllerModelAndProvider(t *testing.T) {
	controller := analyse(t, "app/Http/Controllers/OrderController.php", `<?php

namespace App\Http\Controllers;

use App\Http\Requests\StoreOrderRequest;
use App\Models\Order;
use App\Services\OrderService;
use Illuminate\Http\JsonResponse;

class OrderController extends Controller
{
    public function __construct(private OrderService $orders) {}

    public function store(StoreOrderRequest $request): JsonResponse
    {
        $order = Order::create($request->validated());
        return response()->json($order);
    }
}
`)
	c := byID(controller.Units)[`App\Http\Controllers\OrderController`]
	require.NotNil(t, c)
	assert.Equal(t, unit.KindType, c.Kind)
	// `Controller` is unqualified and not imported, so it is the one in the
	// file's own namespace, App\Http\Controllers\Controller.
	assert.Equal(t, []unit.Marker{super("Controller")}, c.Markers)
	for _, want := range []unit.Ref{
		{Module: `App\Http\Controllers`, Name: "Controller", Exact: true},
		{Module: `App\Http\Requests`, Name: "StoreOrderRequest", Exact: true},
		{Module: `App\Models`, Name: "Order", Exact: true},
		{Module: `App\Services`, Name: "OrderService", Exact: true},
	} {
		assert.Contains(t, c.Refs, want)
	}
	assert.ElementsMatch(t, []string{`App\Http\Requests\StoreOrderRequest`, `App\Models\Order`, `App\Services\OrderService`, `Illuminate\Http\JsonResponse`}, rawImports(controller))

	model := analyse(t, "app/Models/Order.php", `<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class Order extends Model
{
    use HasFactory;

    protected $fillable = ['total'];

    public function customer(): BelongsTo
    {
        return $this->belongsTo(Customer::class);
    }
}
`)
	m := byID(model.Units)[`App\Models\Order`]
	require.NotNil(t, m)
	assert.Equal(t, []unit.Marker{super("Model")}, m.Markers)
	assert.Contains(t, m.Refs, unit.Ref{Module: `App\Models`, Name: "Customer", Exact: true})
	assert.Contains(t, m.Refs, unit.Ref{Module: `Illuminate\Database\Eloquent`, Name: "Model", Exact: true})
	assert.Contains(t, rawImports(model), `Illuminate\Database\Eloquent\Model`)

	provider := analyse(t, "app/Providers/AppServiceProvider.php", `<?php

namespace App\Providers;

use App\Services\OrderService;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(OrderService::class);
    }
}
`)
	p := byID(provider.Units)[`App\Providers\AppServiceProvider`]
	require.NotNil(t, p)
	assert.Equal(t, []unit.Marker{super("ServiceProvider")}, p.Markers)
	assert.Contains(t, p.Refs, unit.Ref{Module: `App\Services`, Name: "OrderService", Exact: true})
}

// Middleware, a Job and a FormRequest. Middleware has no base class at all:
// its role is in the directory, which is the UI's to read from the file.
func TestLaravelMiddlewareJobsAndRequests(t *testing.T) {
	mw := analyse(t, "app/Http/Middleware/EnsureTenant.php", `<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

class EnsureTenant
{
    public function handle(Request $request, Closure $next): Response
    {
        return $next($request);
    }
}
`)
	e := byID(mw.Units)[`App\Http\Middleware\EnsureTenant`]
	require.NotNil(t, e)
	assert.Empty(t, e.Markers)
	assert.Equal(t, []string{"app/Http/Middleware/EnsureTenant.php"}, e.Files)
	// Laravel is built on Symfony components, and a middleware imports one;
	// detection must not read that as a Symfony application.
	assert.Contains(t, rawImports(mw), `Symfony\Component\HttpFoundation\Response`)

	job := analyse(t, "app/Jobs/ProcessOrder.php", `<?php

namespace App\Jobs;

use App\Models\Order;
use App\Services\OrderService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;

class ProcessOrder implements ShouldQueue
{
    use Dispatchable, Queueable;

    public function __construct(public Order $order) {}

    public function handle(OrderService $orders): void
    {
        $orders->process($this->order);
    }
}
`)
	j := byID(job.Units)[`App\Jobs\ProcessOrder`]
	require.NotNil(t, j)
	assert.Equal(t, []unit.Marker{super("ShouldQueue")}, j.Markers)
	assert.Contains(t, j.Refs, unit.Ref{Module: `App\Services`, Name: "OrderService", Exact: true})
	assert.Contains(t, j.Refs, unit.Ref{Module: `App\Models`, Name: "Order", Exact: true})

	req := analyse(t, "app/Http/Requests/StoreOrderRequest.php", `<?php

namespace App\Http\Requests;

use Illuminate\Foundation\Http\FormRequest;

class StoreOrderRequest extends FormRequest
{
    public function rules(): array
    {
        return ['total' => 'required'];
    }
}
`)
	assert.Equal(t, []unit.Marker{super("FormRequest")}, byID(req.Units)[`App\Http\Requests\StoreOrderRequest`].Markers)
}

// A base class is keyed by what the `use` clause says it is, whether it was
// written fully qualified, aliased, or bare.
func TestSupertypesAreKeyedByTheirResolvedName(t *testing.T) {
	src := `<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model as Eloquent;

class Customer extends Eloquent {}
class Invoice extends \Illuminate\Database\Eloquent\Model {}
`
	units := byID(analyse(t, "app/Models/Customer.php", src).Units)
	assert.Equal(t, []unit.Marker{super("Model")}, units[`App\Models\Customer`].Markers, "an alias resolves to the class it names")
	assert.Equal(t, []unit.Marker{super("Model")}, units[`App\Models\Invoice`].Markers, "a fully qualified base is known by its last segment")
}

// A facade is a static call on an imported class; it is a reference to
// something outside the codebase and marks nothing.
func TestFacadesAreReferencesNotMarkers(t *testing.T) {
	src := `<?php

namespace App\Repositories;

use Illuminate\Support\Facades\DB;

class OrderRepository
{
    public function totals(): array
    {
        return DB::table('orders')->sum('total');
    }
}
`
	r := byID(analyse(t, "app/Repositories/OrderRepository.php", src).Units)[`App\Repositories\OrderRepository`]
	require.NotNil(t, r)
	assert.Empty(t, r.Markers)
	assert.Contains(t, r.Refs, unit.Ref{Module: `Illuminate\Support\Facades`, Name: "DB", Exact: true})
}

func TestSymfonyControllerEntityAndRepository(t *testing.T) {
	controller := analyse(t, "src/Controller/ProductController.php", `<?php

namespace App\Controller;

use App\Entity\Product;
use App\Repository\ProductRepository;
use Symfony\Bundle\FrameworkBundle\Controller\AbstractController;
use Symfony\Component\HttpFoundation\Response;
use Symfony\Component\Routing\Attribute\Route;

class ProductController extends AbstractController
{
    #[Route('/products/{id}', name: 'product_show')]
    public function show(Product $product, ProductRepository $products): Response
    {
        return $this->render('product/show.html.twig', ['product' => $product]);
    }
}
`)
	c := byID(controller.Units)[`App\Controller\ProductController`]
	require.NotNil(t, c)
	// The route is on the action, not the class, and it is the class that
	// the lane reads.
	assert.ElementsMatch(t, []unit.Marker{super("AbstractController"), attribute("Route")}, c.Markers)
	assert.Contains(t, c.Refs, unit.Ref{Module: `App\Entity`, Name: "Product", Exact: true})
	assert.Contains(t, c.Refs, unit.Ref{Module: `App\Repository`, Name: "ProductRepository", Exact: true})
	assert.Contains(t, rawImports(controller), `Symfony\Component\Routing\Attribute\Route`)

	entity := analyse(t, "src/Entity/Product.php", `<?php

namespace App\Entity;

use App\Repository\ProductRepository;
use Doctrine\ORM\Mapping as ORM;

#[ORM\Entity(repositoryClass: ProductRepository::class)]
#[ORM\Table(name: 'product')]
class Product
{
    #[ORM\Id]
    #[ORM\GeneratedValue]
    #[ORM\Column]
    private ?int $id = null;

    #[ORM\Column(length: 255)]
    private string $name;

    #[ORM\ManyToOne(targetEntity: Category::class)]
    private ?Category $category = null;
}
`)
	e := byID(entity.Units)[`App\Entity\Product`]
	require.NotNil(t, e)
	// The property attributes are the entity's too, once each; twenty
	// columns are one fact.
	assert.ElementsMatch(t, []unit.Marker{attribute("Entity"), attribute("Table"), attribute("Id"), attribute("GeneratedValue"), attribute("Column"), attribute("ManyToOne")}, e.Markers)
	assert.Contains(t, e.Refs, unit.Ref{Module: `App\Entity`, Name: "Category", Exact: true})
	// The planted violation: an entity naming its repository, as Doctrine's
	// repositoryClass makes every entity do. The UI decides what it means.
	assert.Contains(t, e.Refs, unit.Ref{Module: `App\Repository`, Name: "ProductRepository", Exact: true})

	docblock := analyse(t, "src/Entity/Legacy.php", `<?php

namespace App\Entity;

use Doctrine\ORM\Mapping as ORM;

/**
 * @ORM\Entity
 * @ORM\Table(name="legacy")
 */
class Legacy
{
    /**
     * @ORM\Id
     * @ORM\Column(type="integer")
     */
    private $id;
}
`)
	l := byID(docblock.Units)[`App\Entity\Legacy`]
	require.NotNil(t, l)
	// Only the docblock above the class is read; property docblocks are not.
	assert.ElementsMatch(t, []unit.Marker{attribute("Entity"), attribute("Table")}, l.Markers)

	repo := analyse(t, "src/Repository/ProductRepository.php", `<?php

namespace App\Repository;

use App\Entity\Product;
use Doctrine\Bundle\DoctrineBundle\Repository\ServiceEntityRepository;
use Doctrine\Persistence\ManagerRegistry;

class ProductRepository extends ServiceEntityRepository
{
    public function __construct(ManagerRegistry $registry)
    {
        parent::__construct($registry, Product::class);
    }
}
`)
	r := byID(repo.Units)[`App\Repository\ProductRepository`]
	require.NotNil(t, r)
	assert.Equal(t, []unit.Marker{super("ServiceEntityRepository")}, r.Markers)
	assert.Contains(t, r.Refs, unit.Ref{Module: `App\Entity`, Name: "Product", Exact: true})
}

func TestSymfonySubscribersHandlersAndBundles(t *testing.T) {
	sub := analyse(t, "src/EventSubscriber/OrderSubscriber.php", `<?php

namespace App\EventSubscriber;

use App\Service\OrderMailer;
use Symfony\Component\EventDispatcher\EventSubscriberInterface;

final class OrderSubscriber implements EventSubscriberInterface
{
    public function __construct(private OrderMailer $mailer) {}

    public static function getSubscribedEvents(): array
    {
        return [];
    }
}
`)
	s := byID(sub.Units)[`App\EventSubscriber\OrderSubscriber`]
	require.NotNil(t, s)
	assert.Equal(t, []unit.Marker{super("EventSubscriberInterface")}, s.Markers)
	assert.Contains(t, s.Refs, unit.Ref{Module: `App\Service`, Name: "OrderMailer", Exact: true})

	handler := analyse(t, "src/MessageHandler/PlaceOrderHandler.php", `<?php

namespace App\MessageHandler;

use App\Message\PlaceOrder;
use App\Repository\ProductRepository;
use Symfony\Component\Messenger\Attribute\AsMessageHandler;

#[AsMessageHandler]
final class PlaceOrderHandler
{
    public function __construct(private ProductRepository $products) {}

    public function __invoke(PlaceOrder $message): void {}
}
`)
	h := byID(handler.Units)[`App\MessageHandler\PlaceOrderHandler`]
	require.NotNil(t, h)
	assert.Equal(t, []unit.Marker{attribute("AsMessageHandler")}, h.Markers)
	assert.Contains(t, h.Refs, unit.Ref{Module: `App\Repository`, Name: "ProductRepository", Exact: true})

	bundle := analyse(t, "src/AcmeShopBundle.php", `<?php

namespace Acme\ShopBundle;

use Symfony\Component\HttpKernel\Bundle\AbstractBundle;

class AcmeShopBundle extends AbstractBundle {}
`)
	assert.Equal(t, []unit.Marker{super("AbstractBundle")}, byID(bundle.Units)[`Acme\ShopBundle\AcmeShopBundle`].Markers)
}

// Sylius maps its models to Doctrine in XML, so a model has no attribute.
// It is an interface or a class under Model/, and the interface is marked as
// one so the UI can tell them apart.
func TestSyliusStyleModelsAndInterfaces(t *testing.T) {
	src := `<?php

namespace Sylius\Component\Product\Model;

use Sylius\Component\Resource\Model\ResourceInterface;
use Sylius\Component\Resource\Model\TimestampableInterface;

interface ProductInterface extends ResourceInterface, TimestampableInterface
{
    public function getCode(): ?string;
}

class Product implements ProductInterface
{
    protected ?string $code = null;

    public function getCode(): ?string
    {
        return $this->code;
    }
}

trait ProductTranslationsAwareTrait {}
`
	units := byID(analyse(t, "src/Sylius/Component/Product/Model/Product.php", src).Units)
	i := units[`Sylius\Component\Product\Model\ProductInterface`]
	require.NotNil(t, i)
	assert.ElementsMatch(t, []unit.Marker{super("interface"), super("ResourceInterface"), super("TimestampableInterface")}, i.Markers)
	p := units[`Sylius\Component\Product\Model\Product`]
	require.NotNil(t, p)
	assert.Equal(t, []unit.Marker{super("ProductInterface")}, p.Markers)
	assert.Contains(t, p.Refs, unit.Ref{Module: `Sylius\Component\Product\Model`, Name: "ProductInterface", Exact: true})
	assert.Empty(t, units[`Sylius\Component\Product\Model\ProductTranslationsAwareTrait`].Markers)
}

// An attribute is keyed by the class it resolves to, as a base class is.
func TestAttributesResolveThroughAliases(t *testing.T) {
	src := `<?php

namespace App\Controller;

use Symfony\Component\Routing\Attribute\Route as R;

#[R('/health')]
class HealthController
{
    #[\Symfony\Component\Routing\Attribute\Route('/ping')]
    public function ping(): void {}
}
`
	c := byID(analyse(t, "src/Controller/HealthController.php", src).Units)[`App\Controller\HealthController`]
	require.NotNil(t, c)
	assert.Equal(t, []unit.Marker{attribute("Route")}, c.Markers)
}
