// The shape nopCommerce and every .NET 6+ template are written in: the
// namespace is declared once for the whole file, without a block.
global using System.Text;
using System;
using System.Collections.Generic;
using Nop.Core.Domain.Catalog;
using static System.Math;
using CatalogAlias = Nop.Core.Domain.Catalog;

namespace Nop.Services.Catalog;

public interface IProductService
{
    Task<Product> GetProductByIdAsync(int productId);
}

public class ProductService : IProductService
{
    public async Task<Product> GetProductByIdAsync(int productId)
    {
        return await Task.FromResult(new Product());
    }
}

public record ProductSummary(int Id, string Name);

public struct PriceRange
{
    public decimal From;
    public decimal To;
}
