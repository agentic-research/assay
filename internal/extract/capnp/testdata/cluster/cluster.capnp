# cluster.capnp — test fixture derived from cloister's real cluster.capnp.
# A const Cluster value: bundles declare services; wires declare who talks to
# whom. The sibling manifest schema (manifest/cluster.capnp) is NOT here — this
# is pure CONST DATA, the parseable shape the extractor mines.

@0xb1b1b1b1b1b1b1b1;
using Cluster = import "/cloister/manifest/cluster.capnp";

const cluster :Cluster.Cluster = (
  metadata = ( name = "art-default", version = "0.1.0" ),

  bundles = [
    ( name = "cloister-router",
      description = "Gateway + Durable Object state",
      workerdServiceName = "cloister",
      kind = (external = ( image = "cloister:0.1.0" )),
    ),
    ( name = "notme-identity",
      description = "Identity authority",
      workerdServiceName = "",
      kind = (external = ( image = "notme:0.1.0" )),
    ),
    ( name = "mache",
      description = "Code intelligence",
      workerdServiceName = "",
      kind = (external = ( image = "mache:0.8.0" )),
    ),
    ( name = "rosary",
      description = "Bead orchestrator",
      workerdServiceName = "",
      kind = (external = ( image = "rosary:0.2.0" )),
    ),
  ],

  wires = [
    ( from = "cloister-router", to = "mache",
      binding = "MACHE_BUNDLE",
      transport = (uds = void) ),
    ( from = "cloister-router", to = "rosary",
      binding = "ROSARY_BUNDLE",
      transport = (uds = void) ),
    ( from = "cloister-router", to = "notme-identity",
      binding = "NOTME",
      transport = (uds = void) ),
  ],
);
