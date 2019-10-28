docker.image("consul")
  .withRun { ct_consul ->
  buildGoProject(
    "1.12",
    "--link ${ct_consul.id}:consul",
    "-e CONSUL_HTTP_ADDR=http://consul:8500",
  )
}

