namespace Acme.Core.Contracts;

[Serializable]
public class OrderRequest { }

public interface IOrderHandler { }

public record OrderPlaced(string Id);

public enum OrderState { New, Paid }
