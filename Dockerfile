FROM alpine 

COPY ./csi-plugin /csi-plugin

ENTRYPOINT ["/csi-plugin"]
