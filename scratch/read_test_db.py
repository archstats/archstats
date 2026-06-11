import sqlite3

conn = sqlite3.connect("test.db")
cursor = conn.cursor()

cursor.execute("SELECT name, codesmells__code_health FROM files WHERE (name LIKE '%.json' OR name LIKE '%.yaml' OR name LIKE '%.yml' OR name LIKE '%.lock' OR name LIKE '%node_modules%') AND codesmells__code_health IS NOT NULL")
for row in cursor.fetchall():
    print(row)

conn.close()
