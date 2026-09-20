package com.example.pipeline;

import org.apache.beam.sdk.transforms.PTransform;
import org.apache.beam.sdk.transforms.DoFn;
import jakarta.ws.rs.Path;

@Path("/orders")
@Singleton
public class OrderResource extends PTransform<PCollection<String>, PCollection<Order>> implements Serializable, Comparable<OrderResource> {

    static class ParseFn extends DoFn<String, Order> {
        @ProcessElement
        public void process() {}
    }

    interface Options extends org.apache.beam.sdk.options.PipelineOptions, Runnable {}

    enum Kind { A, B }
}
