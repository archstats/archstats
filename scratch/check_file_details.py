import sqlite3

conn = sqlite3.connect("test.db")
conn.row_factory = sqlite3.Row
cursor = conn.cursor()

cursor.execute("SELECT * FROM files WHERE name = './package.json'")
row = cursor.fetchone()
if row:
    print("package.json stats:")
    for key in row.keys():
        if row[key] is not None:
            print(f"  {key}: {row[key]} ({type(row[key])})")
else:
    print("package.json not found")

conn.close()
