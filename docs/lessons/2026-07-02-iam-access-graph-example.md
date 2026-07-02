# IAM Access Graph Example Lesson

When porting JVM graph examples to Go, keep the first package backend-neutral
until adapter contracts prove themselves. The useful Go shape for #368 is an
immutable fixture plus caller-valued traversal methods over `graph` values, not
a repository/session abstraction.

For README diagrams, start with the reader question instead of showing every
source fixture edge. The IAM diagram stays readable by showing inherited allow,
explicit deny, nested admin drift, and temporary break-glass grant paths while
the tests and README table cover Bob's direct read-only path.
