// Package mariadbgis는 MariaDB의 Point 값과 database/sql용 공간 SQL helper를 제공한다.
//
// MariaDB의 SRID는 geometry 값에 연결된 정수이며, column에는
// REF_SYSTEM_ID 제약을 명시한다. 다른 database dialect와 공통 abstraction은
// 제공하지 않는다.
package mariadbgis
