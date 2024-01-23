FROM envoyproxy/envoy:v1.24.0

# Create the directory for logging
RUN mkdir -p /app/protos

# Make the directory writable by the Envoy process
RUN chmod a+w /app/protos

# Copy protobuf files
COPY ./helloworld.proto /app/protos/
COPY ./helloworld.pb /app/protos/
COPY ./book.proto /app/protos/
COPY ./book.pb /app/protos/

CMD ["/usr/local/bin/envoy", "-c", "/etc/envoy/envoy.yaml"]
