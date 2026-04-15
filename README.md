# proto-sqlite

Elevating sqlite into a fully protobuf-encoded, grpc-compatible interface similar to googlesql

## Instructions

We're encoding sqlite's language / structure into protobuf at a deep, language-spec level. For example:

The base production rule / parse structure is sql-stmt-list, consisting of sql-stmt;sql-stmt... from 0 to any number of statements. sql-stmt can consist of a EXPLAIN [ QUERY PLAN ] followed by one of the following:

alter-table-stmt analyze-stmt attach-stmt begin-stmt commit-stmt create-index-stmt create-table-stmt create-trigger-stmt create-view-stmt create-virtual-table-stmt delete-stmt delete-stmt-limited detach-stmt drop-index-stmt drop-table-stmt drop-trigger-stmt drop-view-stmt insert-stmt pragma-stmt reindex-stmt release-stmt rollback-stmt savepoint-stmt select-stmt update-stmt update-stmt-limited vacuum-stmt

These each have their own structure, saved at eg https://sqlite.org/syntax/alter-table-stmt.html -  a full list of sqlite documentation links is included in sqlite-doc-urls.csv, but all the stmt docs will have this structure.

On these pages, the structure is encoded in an svg/html representation <img width="1237" height="858" alt="Screenshot 2026-04-15 at 3 02 48 PM" src="https://github.com/user-attachments/assets/2aacf208-5356-41ba-bede-ff7b76eb88dd" />

1. We'll need to take screenshots and feed these to multimodal modals to help transcribe these into the full set of production rules. Perhaps it's possible by parsing the svgs - they are structured, but I think it's a lost cause once loops and more complicated relationships get involved. Let's try to extract a bounding box-clipped image for each parse tree into docs/sqlite-parse-img/NAME.png

Note: in these screenshots, if it's listed in References, it's a production rule, otherwise, it's usually a user provided string

2. Then, we will attempt to convert the sqlite spec methodically into the LexDescriptorProto / GrammarDescriptorProto structures found in github.com/accretional/gluon and use that to lex and parse the language as well as formally encode it. Save these as sqlite-lex.textproto and sqlite-grammar.textproto

3. I'll suggest a new pattern not seen in the gluon repo, which I believe googlesql uses: we should also create an enum for all the terminal keywords in sqlite, called keywords.proto, and one for its symbols, called symbols.proto. To try an experiment we'll also create an empty message (eg message ALTER {}) for each keyword in keywords.proto as well as one for each symbol in symbols.proto

4. We can try encoding each stmt as eg https://sqlite.org/syntax/alter-table-stmt.html message AlterTable { Alter alter = 1; Table table = 2; string schema_name = 3; Dot dot = 4; string table_name = 5; ... } with intermediate parses like column-def being encoded as their respective message types eg message ColumnDef{ string column_name = 1; string type_name = 2; repeated ColumnConstraint = 3;}

5. We'll implemnt a grpc server for this in golang where we go:embed the sqlite binary file (include a script for downloading it into this repo during the setup.sh / checking for it in build.sh) and an example sqlite db file and call service Sqlite { rpc Query(SqlStmtList) returns (... } - 

Make sure you create a setup.sh, build.sh, test.sh, and LET_IT_RIP.sh that contain all project setup scripts/commands used - NEVER build/test/run the code in this repo outside of these scripts, NEVER commit or push without running these either. Make them idempotent so that each build.sh can run setup.sh and skip things already set up, each test.sh can run build.sh, each LET_IT_RIP runs test.sh

Implement the parsing logic in lang/ and the protobuf service in sqlite/

Take screenshots of the sqlite urls with https://github.com/accretional/chrome-rpc

Have LET_IT_RIP.sh run queries over the actual embedded sqlite db.

use go 1.26.
