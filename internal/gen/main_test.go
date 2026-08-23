package main

import "testing"

func TestGroupName(t *testing.T) {
	cases := map[string]string{
		"Profiles":       "profiles",
		"Custom Objects": "custom-objects",
		"Web Feeds":      "web-feeds",
	}
	for tag, want := range cases {
		if got := groupName(tag); got != want {
			t.Errorf("groupName(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestCommandName(t *testing.T) {
	cases := []struct{ group, opID, want string }{
		{"profiles", "get_profiles", "list"},
		{"profiles", "get_profile", "get"},
		{"profiles", "create_profile", "create"},
		{"profiles", "update_profile", "update"},
		{"campaigns", "delete_campaign", "delete"},
		{"custom-objects", "get_custom_objects", "list"},
		{"profiles", "get_lists_for_profile", "get-lists-for-profile"},
		{"profiles", "bulk_import_profiles", "bulk-import-profiles"},
		{"campaigns", "get_campaign_message", "get-campaign-message"},
	}
	for _, c := range cases {
		if got := commandName(c.group, c.opID); got != c.want {
			t.Errorf("commandName(%q, %q) = %q, want %q", c.group, c.opID, got, c.want)
		}
	}
}

func TestFlagName(t *testing.T) {
	cases := map[string]string{
		"page[size]":                 "page-size",
		"page[cursor]":               "page-cursor",
		"fields[catalog-variant]":    "fields-catalog-variant",
		"additional-fields[profile]": "additional-fields-profile",
		"company_id":                 "company-id",
		"filter":                     "filter",
		"revision":                   "param-revision", // reserved by a global flag
	}
	for name, want := range cases {
		if got := flagName(name); got != want {
			t.Errorf("flagName(%q) = %q, want %q", name, got, want)
		}
	}
}
