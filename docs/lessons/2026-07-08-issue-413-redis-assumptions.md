# Lesson: Redis Probabilistic Assumption Docs

When Redis probabilistic README diagrams are refreshed, describe the implemented
Redis command contract explicitly instead of implying a Redis module dependency.

- Bloom and HyperLogLog currently use core Redis commands only. Cuckoo `CF*`
  commands should stay documented as follow-up scope until the module,
  persistence, ACL, and Testcontainers contracts exist.
- Package README guidance should name the exported constructors that inherit the
  assumption, then tie operator requirements to the same Redis image used by
  package tests.
- A diagram can pass connector audits while still looking visually weak. Run the
  full-size PNG eye check after coordinate changes and make sure every connector
  visibly starts and ends at the intended card boundary.
