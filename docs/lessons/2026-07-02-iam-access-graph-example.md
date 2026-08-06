# IAM access graph example 교훈

JVM graph example을 Go로 옮길 때는 adapter contract가 증명되기 전까지 첫 package를
backend-neutral하게 유지한다. #368에서 유용한 Go 형태는 repository/session abstraction이
아니라 immutable fixture와 `graph` value 위의 caller-valued traversal method다.

README diagram은 모든 source fixture edge를 보여 주기보다 reader question에서 시작한다.
IAM diagram은 inherited allow, explicit deny, nested admin drift, temporary
break-glass grant 경로를 보여 줄 때 읽기 좋다. Bob의 direct read-only path는 test와
README table이 보완한다.

향후 graph example은 backend adapter보다 domain question을 먼저 고정한다. fixture는 작은
권한 변화가 traversal result를 어떻게 바꾸는지 보여야 하며, diagram은 edge 전체 목록이 아니라
caller가 판단해야 하는 경로를 드러내야 한다. backend-neutral example이 충분히 읽히기 전에는
Neo4j나 다른 저장소 adapter를 public contract처럼 보이게 하지 않는다.
