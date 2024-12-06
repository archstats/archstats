public interface IControl
{
    void Paint();
}
public interface ISurface
{
    void Paint();
}
public abstract class A
{
    public abstract void DoWork(int i);
}
public class SampleClass : IControl, ISurface
{
    public void Paint()
    {
        Console.WriteLine("Paint method in SampleClass");
    }
}

public struct Coords
{
    public Coords(double x, double y)
    {
        X = x;
        Y = y;
    }

    public double X { get; }
    public double Y { get; }

    public override string ToString() => $"({X}, {Y})";
}

public record struct Point
{
    public double X { get; init; }
    public double Y { get; init; }
    public double Z { get; init; }
}