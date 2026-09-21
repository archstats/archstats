namespace Acme.Core.Domain;

public partial class Customer : BaseEntity, ISoftDeletable
{
    public string Email { get; set; }
}
