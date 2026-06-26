# manifest.capnp — a pure SCHEMA fixture: struct/enum/annotation/interface
# definitions only, no `const` data instance. The extractor must emit NOTHING
# for this file (no false positives from struct field names like `name`/`image`).

@0x801fbcc157921d1a;

annotation tier @0xdeadbeef00000001 (field) :Text;

enum Tier {
  hypervisor @0;
  cluster @1;
}

struct Bundle {
  name        @0 :Text;
  description @1 :Text;
  image       @2 :Text;
  tier        @3 :Tier;
}

struct Wire {
  from @0 :Text;
  to   @1 :Text;
}

interface Cluster {
  bundles @0 () -> (list :List(Bundle));
}
