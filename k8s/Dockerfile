FROM golang

RUN git clone https://github.com/moratsam/distry.git
RUN mv distry src/
WORKDIR src/distry/
RUN go get -u ./...
WORKDIR bash/
EXPOSE 53338
EXPOSE 53339
ENTRYPOINT ["bash", "prima"]

