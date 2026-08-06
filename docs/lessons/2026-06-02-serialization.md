# Serialization

Issue #12는 compression work 전에 `serialization`을 safe binary contract layer로
세운다. 기본값은 dependency-free와 explicit behavior를 유지한다. JSON은
`encoding/json`, raw byte/string serializer는 copy-safe utility, persisted payload는
작은 versioned envelope을 사용한다. JVM-style object deserialization이나
reflection-heavy binary format은 별도 security review 없이는 도입하지 않는다.
