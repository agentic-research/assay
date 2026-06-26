# config.capnp — test fixture derived from cloister's real config.capnp.
# A const Workerd.Config value: the worker it serves (a service producer), the
# external upstreams it routes to, and the service bindings that target them.

using Workerd = import "/workerd/workerd.capnp";

const config :Workerd.Config = (
  services = [
    ( name = "cloister",
      worker = .cloisterWorker,
    ),
    ( name = "mache-mcp",
      external = ( address = "127.0.0.1:7532", http = () ),
    ),
    ( name = "rosary-mcp",
      external = ( address = "127.0.0.1:8383", http = () ),
    ),
    ( name = "notme-bot",
      network = ( allow = ["public"] ),
    ),
  ],
);

const cloisterWorker :Workerd.Worker = (
  bindings = [
    ( name = "NOTME",
      service = "notme-bot",
    ),
    ( name = "MACHE_MCP",
      service = "mache-mcp",
    ),
    ( name = "ROSARY_MCP",
      service = "rosary-mcp",
    ),
    ( name = "BEAD_STORE",
      durableObjectNamespace = "BeadStore",
    ),
  ],
);
