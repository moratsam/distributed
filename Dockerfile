FROM golang

RUN git clone https://github.com/moratsam/distributed.git
RUN mv distributed src/
WORKDIR src/distributed/
RUN go get -u ./...
WORKDIR bash/
EXPOSE 53338
EXPOSE 53339
ENTRYPOINT ["bash", "prima"]

