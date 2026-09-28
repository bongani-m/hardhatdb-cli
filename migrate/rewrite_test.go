package migrate

import "testing"

func TestRewriteCreateTable(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "definer engine charset and secondary key",
			in: "CREATE DEFINER=`root`@`localhost` TABLE `accounts` (\n" +
				"  `id` int NOT NULL AUTO_INCREMENT,\n" +
				"  `name` varchar(255) DEFAULT NULL,\n" +
				"  PRIMARY KEY (`id`),\n" +
				"  KEY `name` (`name`)\n" +
				") ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
			want: "CREATE TABLE `accounts` (\n" +
				"  `id` int NOT NULL AUTO_INCREMENT,\n" +
				"  `name` varchar(255) DEFAULT NULL,\n" +
				"  PRIMARY KEY (`id`),\n" +
				"  KEY `name` (`name`)\n" +
				")",
		},
		{
			name: "column comment stays and table comment goes",
			in: "CREATE TABLE `t` (\n" +
				"  `name` varchar(255) COMMENT 'keep me',\n" +
				"  KEY `name` (`name`)\n" +
				") ENGINE=InnoDB COMMENT='drop me'",
			want: "CREATE TABLE `t` (\n" +
				"  `name` varchar(255) COMMENT 'keep me',\n" +
				"  KEY `name` (`name`)\n" +
				")",
		},
		{
			name: "table comment can contain a parenthesis",
			in:   "CREATE TABLE `t` (\n  `id` int\n) COMMENT='a ) b' ROW_FORMAT=DYNAMIC",
			want: "CREATE TABLE `t` (\n  `id` int\n)",
		},
		{
			name: "character set and current user definer",
			in:   "CREATE DEFINER=CURRENT_USER() TABLE `t` (\n  `id` int\n) DEFAULT CHARACTER SET=utf8mb4",
			want: "CREATE TABLE `t` (\n  `id` int\n)",
		},
		{
			name: "partition clause stays",
			in:   "CREATE TABLE `t` (\n  `id` int,\n  KEY `id` (`id`)\n) ENGINE=InnoDB PARTITION BY HASH (`id`)",
			want: "CREATE TABLE `t` (\n  `id` int,\n  KEY `id` (`id`)\n) PARTITION BY HASH (`id`)",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := RewriteCreateTable(test.in)
			if got != test.want {
				t.Fatalf("got\n%s\nwant\n%s", got, test.want)
			}
		})
	}
}

func TestOrderTables(t *testing.T) {
	got := orderTables([]string{"notes", "accounts"}, map[string][]string{
		"notes": {"accounts"},
	})
	want := []string{"accounts", "notes"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}

	cycle := orderTables([]string{"b", "a"}, map[string][]string{
		"a": {"b"},
		"b": {"a"},
	})
	if len(cycle) != 2 || cycle[0] != "a" || cycle[1] != "b" {
		t.Fatalf("cycle order = %v", cycle)
	}
}
