# Klaviyo CLI (beta)

The Klaviyo CLI lets you build, test, and manage your [Klaviyo](https://www.klaviyo.com) integration from the terminal.

With the CLI, you can:

- Call every Klaviyo API operation as a typed command (generated from the [OpenAPI spec](https://github.com/klaviyo/openapi))
- Authenticate once per account and switch between accounts
- Script against the API with built-in jq filtering and cursor pagination
- Hit any endpoint raw with `klaviyo api`, including ones newer than your CLI build

## Installation

**Binary releases** (amd64 and arm64) are on the [Releases page](https://github.com/klaviyo/klaviyo-cli/releases/latest). The commands below fetch them with the [GitHub CLI](https://cli.github.com); downloading the archive for your platform from the Releases page by hand and unpacking it the same way works too.

**macOS**

```bash
gh release download -R klaviyo/klaviyo-cli --pattern '*darwin_arm64*'   # Intel Macs: darwin_amd64
sudo tar -xzf klaviyo-cli_*_darwin_arm64.tar.gz -C /usr/local/bin klaviyo   # or -C any directory on your PATH
```

The binaries are not notarized yet, so if you download with a browser instead of `gh`, macOS quarantines the file — clear it with `sudo xattr -d com.apple.quarantine /usr/local/bin/klaviyo`.

**Linux**

```bash
gh release download -R klaviyo/klaviyo-cli --pattern '*linux_amd64*'    # ARM: linux_arm64
sudo tar -xzf klaviyo-cli_*_linux_amd64.tar.gz -C /usr/local/bin klaviyo    # or -C any directory on your PATH
```

**Windows** (PowerShell)

```powershell
gh release download -R klaviyo/klaviyo-cli --pattern '*windows_amd64*'  # ARM: windows_arm64
Expand-Archive klaviyo-cli_*_windows_amd64.zip -DestinationPath klaviyo-cli-release
Move-Item klaviyo-cli-release\klaviyo.exe "$env:LOCALAPPDATA\Microsoft\WindowsApps\"   # or any directory on your PATH
```

**From source** (Go 1.25+):

```bash
go install github.com/klaviyo/klaviyo-cli/cmd/klaviyo@latest
```

### Upgrading

The CLI checks GitHub for a newer release at most once per day and prints a notice to stderr — only on interactive terminals, never in CI or when piped. It never self-updates: upgrade through the channel you installed with. Opt out of the check with `KLAVIYO_NO_UPDATE_NOTIFIER=1`.

## Quickstart

```bash
# Store a private API key for an account (verified before saving)
klaviyo auth login

# Confirm credentials
klaviyo auth status

# Typed commands for every API operation
klaviyo metrics list
klaviyo profiles list --filter 'equals(email,"someone@example.com")'
klaviyo profiles get 01ABC123 --fields-profile email,first_name
klaviyo lists list --paginate          # follow cursors, merge all pages
klaviyo lists create --name Newsletter # body attributes are flags
klaviyo profiles create --email someone@example.com --location.city Boston
klaviyo events create -d @event.json   # or supply the whole body yourself

# Filter any response with the built-in jq (no jq install needed)
klaviyo lists list --jq '.data[].attributes.name'
klaviyo profiles list --paginate --jq '.data | length'

# Or call any endpoint raw
klaviyo api /api/metrics/
klaviyo api POST /api/events/ -d @event.json
```

## Commands

Core commands:

| Command | Description |
| --- | --- |
| `klaviyo auth login` | Store an API key for a named account (verified first) |
| `klaviyo auth logout <account>` | Remove an account and its key |
| `klaviyo auth list` | List configured accounts |
| `klaviyo auth switch <account>` | Set the default account |
| `klaviyo auth status` | Verify credentials for the selected account |
| `klaviyo auth migrate` | Move file-stored API keys into the OS keychain |
| `klaviyo api [method] <path>` | Raw authenticated API request (defaults to GET) |
| `klaviyo config` | Show or edit CLI configuration (`--list`, `--set`, `-e`) |
| `klaviyo open <shortcut>` | Open Klaviyo dashboard or docs pages in your browser |
| `klaviyo completion <shell>` | Generate shell completion scripts |
| `klaviyo version` | Print the CLI version |

<!-- klaviyo-cli-gen:commands:begin -->
Resource commands cover every operation in the Klaviyo API — one command per operation, 345 commands across 23 groups. Expand a group for its commands (run `klaviyo <group> <command> --help` for arguments and flags):

<details>
<summary><strong><code>accounts</code></strong> — 2 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo accounts get` | Get Account |
| `klaviyo accounts list` | Get Accounts |

</details>
<details>
<summary><strong><code>campaigns</code></strong> — 26 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo campaigns assign-template-to-campaign-message` | Assign Template to Campaign Message |
| `klaviyo campaigns cancel-campaign-send` | Cancel Campaign Send |
| `klaviyo campaigns create` | Create Campaign |
| `klaviyo campaigns create-campaign-clone` | Create Campaign Clone |
| `klaviyo campaigns delete` | Delete Campaign |
| `klaviyo campaigns get` | Get Campaign |
| `klaviyo campaigns get-campaign-for-campaign-message` | Get Campaign for Campaign Message |
| `klaviyo campaigns get-campaign-id-for-campaign-message` | Get Campaign ID for Campaign Message |
| `klaviyo campaigns get-campaign-message` | Get Campaign Message |
| `klaviyo campaigns get-campaign-recipient-estimation` | Get Campaign Recipient Estimation |
| `klaviyo campaigns get-campaign-recipient-estimation-job` | Get Campaign Recipient Estimation Job |
| `klaviyo campaigns get-campaign-send-job` | Get Campaign Send Job |
| `klaviyo campaigns get-image-for-campaign-message` | Get Image for Campaign Message |
| `klaviyo campaigns get-image-id-for-campaign-message` | Get Image ID for Campaign Message |
| `klaviyo campaigns get-message-ids-for-campaign` | Get Message IDs for Campaign |
| `klaviyo campaigns get-messages-for-campaign` | Get Messages for Campaign |
| `klaviyo campaigns get-tag-ids-for-campaign` | Get Tag IDs for Campaign |
| `klaviyo campaigns get-tags-for-campaign` | Get Tags for Campaign |
| `klaviyo campaigns get-template-for-campaign-message` | Get Template for Campaign Message |
| `klaviyo campaigns get-template-id-for-campaign-message` | Get Template ID for Campaign Message |
| `klaviyo campaigns list` | Get Campaigns |
| `klaviyo campaigns refresh-campaign-recipient-estimation` | Refresh Campaign Recipient Estimation |
| `klaviyo campaigns send-campaign` | Send Campaign |
| `klaviyo campaigns update` | Update Campaign |
| `klaviyo campaigns update-campaign-message` | Update Campaign Message |
| `klaviyo campaigns update-image-for-campaign-message` | Update Image for Campaign Message |

</details>
<details>
<summary><strong><code>catalogs</code></strong> — 55 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo catalogs add-categories-to-catalog-item` | Add Categories to Catalog Item |
| `klaviyo catalogs add-items-to-catalog-category` | Add Items to Catalog Category |
| `klaviyo catalogs bulk-create-catalog-categories` | Bulk Create Catalog Categories |
| `klaviyo catalogs bulk-create-catalog-items` | Bulk Create Catalog Items |
| `klaviyo catalogs bulk-create-catalog-variants` | Bulk Create Catalog Variants |
| `klaviyo catalogs bulk-delete-catalog-categories` | Bulk Delete Catalog Categories |
| `klaviyo catalogs bulk-delete-catalog-items` | Bulk Delete Catalog Items |
| `klaviyo catalogs bulk-delete-catalog-variants` | Bulk Delete Catalog Variants |
| `klaviyo catalogs bulk-update-catalog-categories` | Bulk Update Catalog Categories |
| `klaviyo catalogs bulk-update-catalog-items` | Bulk Update Catalog Items |
| `klaviyo catalogs bulk-update-catalog-variants` | Bulk Update Catalog Variants |
| `klaviyo catalogs create-back-in-stock-subscription` | Create Back In Stock Subscription |
| `klaviyo catalogs create-catalog-category` | Create Catalog Category |
| `klaviyo catalogs create-catalog-item` | Create Catalog Item |
| `klaviyo catalogs create-catalog-variant` | Create Catalog Variant |
| `klaviyo catalogs delete-catalog-category` | Delete Catalog Category |
| `klaviyo catalogs delete-catalog-item` | Delete Catalog Item |
| `klaviyo catalogs delete-catalog-variant` | Delete Catalog Variant |
| `klaviyo catalogs get-bulk-create-catalog-items-job` | Get Bulk Create Catalog Items Job |
| `klaviyo catalogs get-bulk-create-catalog-items-jobs` | Get Bulk Create Catalog Items Jobs |
| `klaviyo catalogs get-bulk-create-categories-job` | Get Bulk Create Categories Job |
| `klaviyo catalogs get-bulk-create-categories-jobs` | Get Bulk Create Categories Jobs |
| `klaviyo catalogs get-bulk-create-variants-job` | Get Bulk Create Variants Job |
| `klaviyo catalogs get-bulk-create-variants-jobs` | Get Bulk Create Variants Jobs |
| `klaviyo catalogs get-bulk-delete-catalog-items-job` | Get Bulk Delete Catalog Items Job |
| `klaviyo catalogs get-bulk-delete-catalog-items-jobs` | Get Bulk Delete Catalog Items Jobs |
| `klaviyo catalogs get-bulk-delete-categories-job` | Get Bulk Delete Categories Job |
| `klaviyo catalogs get-bulk-delete-categories-jobs` | Get Bulk Delete Categories Jobs |
| `klaviyo catalogs get-bulk-delete-variants-job` | Get Bulk Delete Variants Job |
| `klaviyo catalogs get-bulk-delete-variants-jobs` | Get Bulk Delete Variants Jobs |
| `klaviyo catalogs get-bulk-update-catalog-items-job` | Get Bulk Update Catalog Items Job |
| `klaviyo catalogs get-bulk-update-catalog-items-jobs` | Get Bulk Update Catalog Items Jobs |
| `klaviyo catalogs get-bulk-update-categories-job` | Get Bulk Update Categories Job |
| `klaviyo catalogs get-bulk-update-categories-jobs` | Get Bulk Update Categories Jobs |
| `klaviyo catalogs get-bulk-update-variants-job` | Get Bulk Update Variants Job |
| `klaviyo catalogs get-bulk-update-variants-jobs` | Get Bulk Update Variants Jobs |
| `klaviyo catalogs get-catalog-categories` | Get Catalog Categories |
| `klaviyo catalogs get-catalog-category` | Get Catalog Category |
| `klaviyo catalogs get-catalog-item` | Get Catalog Item |
| `klaviyo catalogs get-catalog-items` | Get Catalog Items |
| `klaviyo catalogs get-catalog-variant` | Get Catalog Variant |
| `klaviyo catalogs get-catalog-variants` | Get Catalog Variants |
| `klaviyo catalogs get-categories-for-catalog-item` | Get Categories for Catalog Item |
| `klaviyo catalogs get-category-ids-for-catalog-item` | Get Category IDs for Catalog Item |
| `klaviyo catalogs get-item-ids-for-catalog-category` | Get Item IDs for Catalog Category |
| `klaviyo catalogs get-items-for-catalog-category` | Get Items for Catalog Category |
| `klaviyo catalogs get-variant-ids-for-catalog-item` | Get Variant IDs for Catalog Item |
| `klaviyo catalogs get-variants-for-catalog-item` | Get Variants for Catalog Item |
| `klaviyo catalogs remove-categories-from-catalog-item` | Remove Categories from Catalog Item |
| `klaviyo catalogs remove-items-from-catalog-category` | Remove Items from Catalog Category |
| `klaviyo catalogs update-catalog-category` | Update Catalog Category |
| `klaviyo catalogs update-catalog-item` | Update Catalog Item |
| `klaviyo catalogs update-catalog-variant` | Update Catalog Variant |
| `klaviyo catalogs update-categories-for-catalog-item` | Update Categories for Catalog Item |
| `klaviyo catalogs update-items-for-catalog-category` | Update Items for Catalog Category |

</details>
<details>
<summary><strong><code>client</code></strong> — 12 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo client bulk-create-client-events` | Bulk Create Client Events |
| `klaviyo client create-client-back-in-stock-subscription` | Create Client Back In Stock Subscription |
| `klaviyo client create-client-event` | Create Client Event |
| `klaviyo client create-client-profile` | Create or Update Client Profile |
| `klaviyo client create-client-push-token` | Create or Update Client Push Token |
| `klaviyo client create-client-review` | Create Client Review |
| `klaviyo client create-client-subscription` | Create Client Subscription |
| `klaviyo client get-client-geofences` | Get Client Geofences |
| `klaviyo client get-client-ip-allowlist` | Get Client IP Allowlist |
| `klaviyo client get-client-review-values-reports` | Get Client Review Values Reports |
| `klaviyo client get-client-reviews` | Get Client Reviews |
| `klaviyo client unregister-client-push-token` | Unregister Client Push Token |

</details>
<details>
<summary><strong><code>conversations</code></strong> — 1 command</summary>

| Command | Description |
| --- | --- |
| `klaviyo conversations create-conversation-message` | Create Conversation Message |

</details>
<details>
<summary><strong><code>coupons</code></strong> — 17 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo coupons bulk-create-coupon-codes` | Bulk Create Coupon Codes |
| `klaviyo coupons create` | Create Coupon |
| `klaviyo coupons create-coupon-code` | Create Coupon Code |
| `klaviyo coupons delete` | Delete Coupon |
| `klaviyo coupons delete-coupon-code` | Delete Coupon Code |
| `klaviyo coupons get` | Get Coupon |
| `klaviyo coupons get-bulk-create-coupon-code-jobs` | Get Bulk Create Coupon Code Jobs |
| `klaviyo coupons get-bulk-create-coupon-codes-job` | Get Bulk Create Coupon Codes Job |
| `klaviyo coupons get-coupon-code` | Get Coupon Code |
| `klaviyo coupons get-coupon-code-ids-for-coupon` | Get Coupon Code IDs for Coupon |
| `klaviyo coupons get-coupon-codes` | Get Coupon Codes |
| `klaviyo coupons get-coupon-codes-for-coupon` | Get Coupon Codes for Coupon |
| `klaviyo coupons get-coupon-for-coupon-code` | Get Coupon For Coupon Code |
| `klaviyo coupons get-coupon-id-for-coupon-code` | Get Coupon ID for Coupon Code |
| `klaviyo coupons list` | Get Coupons |
| `klaviyo coupons update` | Update Coupon |
| `klaviyo coupons update-coupon-code` | Update Coupon Code |

</details>
<details>
<summary><strong><code>custom-objects</code></strong> — 39 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo custom-objects bulk-create-data-source-records` | Bulk Create Data Source Records |
| `klaviyo custom-objects bulk-delete-object-records` | Bulk Delete Object Records |
| `klaviyo custom-objects create-data-source` | Create Data Source |
| `klaviyo custom-objects create-data-source-record` | Create Data Source Record |
| `klaviyo custom-objects create-object-schema` | Create Object Schema |
| `klaviyo custom-objects create-object-schema-relationship` | Create Object Schema Relationship |
| `klaviyo custom-objects create-object-type` | Create Object Type |
| `klaviyo custom-objects create-profile-schema-relationship` | Create Profile Schema Relationship |
| `klaviyo custom-objects delete-data-source` | Delete Data Source |
| `klaviyo custom-objects delete-object-schema-relationship` | Delete Object Schema Relationship |
| `klaviyo custom-objects delete-object-type` | Delete Object Type |
| `klaviyo custom-objects delete-profile-schema-relationship` | Delete Profile Schema Relationship |
| `klaviyo custom-objects get-current-schema-for-object-type` | Get Current Schema for Object Type |
| `klaviyo custom-objects get-current-schema-id-for-object-type` | Get Current Schema ID for Object Type |
| `klaviyo custom-objects get-data-source` | Get Data Source |
| `klaviyo custom-objects get-data-sources` | Get Data Sources |
| `klaviyo custom-objects get-draft-schema-for-object-type` | Get Draft Schema for Object Type |
| `klaviyo custom-objects get-draft-schema-id-for-object-type` | Get Draft Schema ID for Object Type |
| `klaviyo custom-objects get-ingestion-log-ids-for-object-type` | Get Ingestion Log IDs for Object Type |
| `klaviyo custom-objects get-ingestion-logs-for-object-type` | Get Ingestion Logs for Object Type |
| `klaviyo custom-objects get-object-record` | Get Object Record |
| `klaviyo custom-objects get-object-schema` | Get Object Schema |
| `klaviyo custom-objects get-object-schema-relationships` | Get Object Schema Relationships |
| `klaviyo custom-objects get-object-type` | Get Object Type |
| `klaviyo custom-objects get-object-type-relationships` | Get Object Type Relationships |
| `klaviyo custom-objects get-object-types` | Get Object Types |
| `klaviyo custom-objects get-profile-schema-relationships` | Get Profile Schema Relationships |
| `klaviyo custom-objects get-profile-type-relationships` | Get Profile Type Relationships |
| `klaviyo custom-objects get-record-ids-for-object-type` | Get Record IDs for Object Type |
| `klaviyo custom-objects get-records-for-object-type` | Get Records for Object Type |
| `klaviyo custom-objects get-schema-version-ids-for-object-type` | Get Schema Version IDs for Object Type |
| `klaviyo custom-objects get-schema-versions-for-object-type` | Get Schema Versions for Object Type |
| `klaviyo custom-objects get-source-mapping` | Get Source Mapping |
| `klaviyo custom-objects get-source-mapping-for-object-schema` | Get Source Mapping for Object Schema |
| `klaviyo custom-objects get-source-mapping-id-for-object-schema` | Get Source Mapping ID for Object Schema |
| `klaviyo custom-objects update-object-schema` | Update Object Schema |
| `klaviyo custom-objects update-object-schema-relationship` | Update Object Schema Relationship |
| `klaviyo custom-objects update-profile-schema-relationship` | Update Profile Schema Relationship |
| `klaviyo custom-objects update-source-mapping` | Update Source Mapping |

</details>
<details>
<summary><strong><code>data-privacy</code></strong> — 1 command</summary>

| Command | Description |
| --- | --- |
| `klaviyo data-privacy request-profile-deletion` | Request Profile Deletion |

</details>
<details>
<summary><strong><code>events</code></strong> — 8 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo events bulk-create-events` | Bulk Create Events |
| `klaviyo events create` | Create Event |
| `klaviyo events get` | Get Event |
| `klaviyo events get-metric-for-event` | Get Metric for Event |
| `klaviyo events get-metric-id-for-event` | Get Metric ID for Event |
| `klaviyo events get-profile-for-event` | Get Profile for Event |
| `klaviyo events get-profile-id-for-event` | Get Profile ID for Event |
| `klaviyo events list` | Get Events |

</details>
<details>
<summary><strong><code>flows</code></strong> — 21 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo flows create` | Create Flow |
| `klaviyo flows delete` | Delete Flow |
| `klaviyo flows delete-flow-action` | Delete Flow Action |
| `klaviyo flows get` | Get Flow |
| `klaviyo flows get-action-for-flow-message` | Get Action for Flow Message |
| `klaviyo flows get-action-id-for-flow-message` | Get Action ID for Flow Message |
| `klaviyo flows get-action-ids-for-flow` | Get Action IDs for Flow |
| `klaviyo flows get-actions-for-flow` | Get Actions for Flow |
| `klaviyo flows get-flow-action` | Get Flow Action |
| `klaviyo flows get-flow-action-messages` | Get Messages For Flow Action |
| `klaviyo flows get-flow-for-flow-action` | Get Flow for Flow Action |
| `klaviyo flows get-flow-id-for-flow-action` | Get Flow ID for Flow Action |
| `klaviyo flows get-flow-message` | Get Flow Message |
| `klaviyo flows get-message-ids-for-flow-action` | Get Message IDs for Flow Action |
| `klaviyo flows get-tag-ids-for-flow` | Get Tag IDs for Flow |
| `klaviyo flows get-tags-for-flow` | Get Tags for Flow |
| `klaviyo flows get-template-for-flow-message` | Get Template for Flow Message |
| `klaviyo flows get-template-id-for-flow-message` | Get Template ID for Flow Message |
| `klaviyo flows list` | Get Flows |
| `klaviyo flows update` | Update Flow Status |
| `klaviyo flows update-flow-action` | Update Flow Action |

</details>
<details>
<summary><strong><code>forms</code></strong> — 9 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo forms create` | Create Form |
| `klaviyo forms delete` | Delete Form |
| `klaviyo forms get` | Get Form |
| `klaviyo forms get-form-for-form-version` | Get Form for Form Version |
| `klaviyo forms get-form-id-for-form-version` | Get Form ID for Form Version |
| `klaviyo forms get-form-version` | Get Form Version |
| `klaviyo forms get-version-ids-for-form` | Get Version IDs for Form |
| `klaviyo forms get-versions-for-form` | Get Versions for Form |
| `klaviyo forms list` | Get Forms |

</details>
<details>
<summary><strong><code>images</code></strong> — 5 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo images get` | Get Image |
| `klaviyo images list` | Get Images |
| `klaviyo images update` | Update Image |
| `klaviyo images upload-image-from-file` | Upload Image From File |
| `klaviyo images upload-image-from-url` | Upload Image From URL |

</details>
<details>
<summary><strong><code>lists</code></strong> — 13 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo lists add-profiles-to-list` | Add Profiles to List |
| `klaviyo lists create` | Create List |
| `klaviyo lists delete` | Delete List |
| `klaviyo lists get` | Get List |
| `klaviyo lists get-flows-triggered-by-list` | Get Flows Triggered by List |
| `klaviyo lists get-ids-for-flows-triggered-by-list` | Get IDs for Flows Triggered by List |
| `klaviyo lists get-profile-ids-for-list` | Get Profile IDs for List |
| `klaviyo lists get-profiles-for-list` | Get Profiles for List |
| `klaviyo lists get-tag-ids-for-list` | Get Tag IDs for List |
| `klaviyo lists get-tags-for-list` | Get Tags for List |
| `klaviyo lists list` | Get Lists |
| `klaviyo lists remove-profiles-from-list` | Remove Profiles from List |
| `klaviyo lists update` | Update List |

</details>
<details>
<summary><strong><code>metrics</code></strong> — 24 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo metrics create-custom-metric` | Create Custom Metric |
| `klaviyo metrics delete-custom-metric` | Delete Custom Metric |
| `klaviyo metrics get` | Get Metric |
| `klaviyo metrics get-custom-metric` | Get Custom Metric |
| `klaviyo metrics get-custom-metric-for-mapped-metric` | Get Custom Metric for Mapped Metric |
| `klaviyo metrics get-custom-metric-id-for-mapped-metric` | Get Custom Metric ID for Mapped Metric |
| `klaviyo metrics get-custom-metrics` | Get Custom Metrics |
| `klaviyo metrics get-flows-triggered-by-metric` | Get Flows Triggered by Metric |
| `klaviyo metrics get-ids-for-flows-triggered-by-metric` | Get IDs for Flows Triggered by Metric |
| `klaviyo metrics get-mapped-metric` | Get Mapped Metric |
| `klaviyo metrics get-mapped-metrics` | Get Mapped Metrics |
| `klaviyo metrics get-metric-for-mapped-metric` | Get Metric for Mapped Metric |
| `klaviyo metrics get-metric-for-metric-property` | Get Metric for Metric Property |
| `klaviyo metrics get-metric-id-for-mapped-metric` | Get Metric ID for Mapped Metric |
| `klaviyo metrics get-metric-id-for-metric-property` | Get Metric ID for Metric Property |
| `klaviyo metrics get-metric-ids-for-custom-metric` | Get Metric IDs for Custom Metric |
| `klaviyo metrics get-metric-property` | Get Metric Property |
| `klaviyo metrics get-metrics-for-custom-metric` | Get Metrics for Custom Metric |
| `klaviyo metrics get-properties-for-metric` | Get Properties for Metric |
| `klaviyo metrics get-property-ids-for-metric` | Get Property IDs for Metric |
| `klaviyo metrics list` | Get Metrics |
| `klaviyo metrics query-metric-aggregates` | Query Metric Aggregates |
| `klaviyo metrics update-custom-metric` | Update Custom Metric |
| `klaviyo metrics update-mapped-metric` | Update Mapped Metric |

</details>
<details>
<summary><strong><code>profiles</code></strong> — 38 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo profiles bulk-import-profiles` | Bulk Import Profiles |
| `klaviyo profiles bulk-subscribe-profiles` | Bulk Subscribe Profiles |
| `klaviyo profiles bulk-suppress-profiles` | Bulk Suppress Profiles |
| `klaviyo profiles bulk-unsubscribe-profiles` | Bulk Unsubscribe Profiles |
| `klaviyo profiles bulk-unsuppress-profiles` | Bulk Unsuppress Profiles |
| `klaviyo profiles create` | Create Profile |
| `klaviyo profiles create-or-update-profile` | Create or Update Profile |
| `klaviyo profiles create-push-token` | Create or Update Push Token |
| `klaviyo profiles delete-push-token` | Delete Push Token |
| `klaviyo profiles get` | Get Profile |
| `klaviyo profiles get-bulk-import-profiles-job` | Get Bulk Import Profiles Job |
| `klaviyo profiles get-bulk-import-profiles-jobs` | Get Bulk Import Profiles Jobs |
| `klaviyo profiles get-bulk-suppress-profiles-job` | Get Bulk Suppress Profiles Job |
| `klaviyo profiles get-bulk-suppress-profiles-jobs` | Get Bulk Suppress Profiles Jobs |
| `klaviyo profiles get-bulk-unsuppress-profiles-job` | Get Bulk Unsuppress Profiles Job |
| `klaviyo profiles get-bulk-unsuppress-profiles-jobs` | Get Bulk Unsuppress Profiles Jobs |
| `klaviyo profiles get-conversation-for-profile` | Get Conversation for Profile |
| `klaviyo profiles get-conversation-id-for-profile` | Get Conversation ID for Profile |
| `klaviyo profiles get-conversation-ids-for-profile` | Get Conversation IDs for Profile |
| `klaviyo profiles get-conversations-for-profile` | Get Conversations for Profile |
| `klaviyo profiles get-errors-for-bulk-import-profiles-job` | Get Errors for Bulk Import Profiles Job |
| `klaviyo profiles get-list-for-bulk-import-profiles-job` | Get List for Bulk Import Profiles Job |
| `klaviyo profiles get-list-ids-for-bulk-import-profiles-job` | Get List IDs for Bulk Import Profiles Job |
| `klaviyo profiles get-list-ids-for-profile` | Get List IDs for Profile |
| `klaviyo profiles get-lists-for-profile` | Get Lists for Profile |
| `klaviyo profiles get-profile-for-push-token` | Get Profile for Push Token |
| `klaviyo profiles get-profile-id-for-push-token` | Get Profile ID for Push Token |
| `klaviyo profiles get-profile-ids-for-bulk-import-profiles-job` | Get Profile IDs for Bulk Import Profiles Job |
| `klaviyo profiles get-profiles-for-bulk-import-profiles-job` | Get Profiles for Bulk Import Profiles Job |
| `klaviyo profiles get-push-token` | Get Push Token |
| `klaviyo profiles get-push-token-ids-for-profile` | Get Push Token IDs for Profile |
| `klaviyo profiles get-push-tokens` | Get Push Tokens |
| `klaviyo profiles get-push-tokens-for-profile` | Get Push Tokens for Profile |
| `klaviyo profiles get-segment-ids-for-profile` | Get Segment IDs for Profile |
| `klaviyo profiles get-segments-for-profile` | Get Segments for Profile |
| `klaviyo profiles list` | Get Profiles |
| `klaviyo profiles merge-profiles` | Merge Profiles |
| `klaviyo profiles update` | Update Profile |

</details>
<details>
<summary><strong><code>reporting</code></strong> — 7 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo reporting query-campaign-values` | Query Campaign Values |
| `klaviyo reporting query-flow-series` | Query Flow Series |
| `klaviyo reporting query-flow-values` | Query Flow Values |
| `klaviyo reporting query-form-series` | Query Form Series |
| `klaviyo reporting query-form-values` | Query Form Values |
| `klaviyo reporting query-segment-series` | Query Segment Series |
| `klaviyo reporting query-segment-values` | Query Segment Values |

</details>
<details>
<summary><strong><code>reviews</code></strong> — 3 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo reviews get` | Get Review |
| `klaviyo reviews list` | Get Reviews |
| `klaviyo reviews update` | Update Review |

</details>
<details>
<summary><strong><code>segments</code></strong> — 11 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo segments create` | Create Segment |
| `klaviyo segments delete` | Delete Segment |
| `klaviyo segments get` | Get Segment |
| `klaviyo segments get-flows-triggered-by-segment` | Get Flows Triggered by Segment |
| `klaviyo segments get-ids-for-flows-triggered-by-segment` | Get IDs for Flows Triggered by Segment |
| `klaviyo segments get-profile-ids-for-segment` | Get Profile IDs for Segment |
| `klaviyo segments get-profiles-for-segment` | Get Profiles for Segment |
| `klaviyo segments get-tag-ids-for-segment` | Get Tag IDs for Segment |
| `klaviyo segments get-tags-for-segment` | Get Tags for Segment |
| `klaviyo segments list` | Get Segments |
| `klaviyo segments update` | Update Segment |

</details>
<details>
<summary><strong><code>tags</code></strong> — 26 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo tags create` | Create Tag |
| `klaviyo tags create-tag-group` | Create Tag Group |
| `klaviyo tags delete` | Delete Tag |
| `klaviyo tags delete-tag-group` | Delete Tag Group |
| `klaviyo tags get` | Get Tag |
| `klaviyo tags get-campaign-ids-for-tag` | Get Campaign IDs for Tag |
| `klaviyo tags get-flow-ids-for-tag` | Get Flow IDs for Tag |
| `klaviyo tags get-list-ids-for-tag` | Get List IDs for Tag |
| `klaviyo tags get-segment-ids-for-tag` | Get Segment IDs for Tag |
| `klaviyo tags get-tag-group` | Get Tag Group |
| `klaviyo tags get-tag-group-for-tag` | Get Tag Group for Tag |
| `klaviyo tags get-tag-group-id-for-tag` | Get Tag Group ID for Tag |
| `klaviyo tags get-tag-groups` | Get Tag Groups |
| `klaviyo tags get-tag-ids-for-tag-group` | Get Tag IDs for Tag Group |
| `klaviyo tags get-tags-for-tag-group` | Get Tags for Tag Group |
| `klaviyo tags list` | Get Tags |
| `klaviyo tags remove-tag-from-campaigns` | Remove Tag from Campaigns |
| `klaviyo tags remove-tag-from-flows` | Remove Tag from Flows |
| `klaviyo tags remove-tag-from-lists` | Remove Tag from Lists |
| `klaviyo tags remove-tag-from-segments` | Remove Tag from Segments |
| `klaviyo tags tag-campaigns` | Tag Campaigns |
| `klaviyo tags tag-flows` | Tag Flows |
| `klaviyo tags tag-lists` | Tag Lists |
| `klaviyo tags tag-segments` | Tag Segments |
| `klaviyo tags update` | Update Tag |
| `klaviyo tags update-tag-group` | Update Tag Group |

</details>
<details>
<summary><strong><code>templates</code></strong> — 12 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo templates clone-template` | Clone Template |
| `klaviyo templates create` | Create Template |
| `klaviyo templates create-universal-content` | Create Universal Content |
| `klaviyo templates delete` | Delete Template |
| `klaviyo templates delete-universal-content` | Delete Universal Content |
| `klaviyo templates get` | Get Template |
| `klaviyo templates get-all-universal-content` | Get All Universal Content |
| `klaviyo templates get-universal-content` | Get Universal Content |
| `klaviyo templates list` | Get Templates |
| `klaviyo templates render-template` | Render Template |
| `klaviyo templates update` | Update Template |
| `klaviyo templates update-universal-content` | Update Universal Content |

</details>
<details>
<summary><strong><code>tracking-settings</code></strong> — 3 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo tracking-settings get` | Get Tracking Setting |
| `klaviyo tracking-settings list` | Get Tracking Settings |
| `klaviyo tracking-settings update` | Update Tracking Setting |

</details>
<details>
<summary><strong><code>web-feeds</code></strong> — 5 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo web-feeds create` | Create Web Feed |
| `klaviyo web-feeds delete` | Delete Web Feed |
| `klaviyo web-feeds get` | Get Web Feed |
| `klaviyo web-feeds list` | Get Web Feeds |
| `klaviyo web-feeds update` | Update Web Feed |

</details>
<details>
<summary><strong><code>webhooks</code></strong> — 7 commands</summary>

| Command | Description |
| --- | --- |
| `klaviyo webhooks create` | Create Webhook |
| `klaviyo webhooks delete` | Delete Webhook |
| `klaviyo webhooks get` | Get Webhook |
| `klaviyo webhooks get-webhook-topic` | Get Webhook Topic |
| `klaviyo webhooks get-webhook-topics` | Get Webhook Topics |
| `klaviyo webhooks list` | Get Webhooks |
| `klaviyo webhooks update` | Update Webhook |

</details>
<!-- klaviyo-cli-gen:commands:end -->

Conventions across all resource commands:

- Canonical CRUD is `list`, `get`, `create`, `update`, `delete`; everything else keeps its operation name (`klaviyo profiles get-lists-for-profile`).
- Path parameters are positional arguments; query parameters are flags (`page[size]` becomes `--page-size`).
- **Body attributes are flags too**: each scalar field under the body's `data.attributes` gets its own typed, documented flag — `klaviyo lists create --name Newsletter --opt-in-process double_opt_in`, with dots for nested objects (`--location.city Boston`) and repeats for arrays. The JSON:API `data.type` is filled in automatically, and `--help` lists every field with its description from the API spec.
- Anything a flag can't express — free-form maps like event `properties`, arrays of objects, relationships, bulk endpoints — uses `-d` / `--data`: repeatable `path=value` pairs where dots nest objects (`-d data.attributes.properties.item=shirt`) and `:=` assigns a JSON value (`-d 'data.relationships.lists.data:=[{"type":"list","id":"Abc123"}]'`), or a single `-d` with inline JSON, `@file`, or `-` for stdin. Body flags and `-d` pairs combine into one body; conflicting fields are an error.
- List commands accept `--paginate` to follow cursors and merge every page's `data` array.

Run `klaviyo <group> --help` for a group's commands, or `klaviyo <group> <command> --help` for its flags. The full reference is the CLI's own help output.

## Authentication and accounts

`klaviyo auth login` verifies a [private API key](https://developers.klaviyo.com/en/docs/authenticate_) against the API, then stores it as a named account profile. The first account added becomes the default. Store as many accounts as you like:

```bash
klaviyo auth login --account prod
klaviyo auth login --account staging
klaviyo auth list                      # * marks the default
klaviyo auth switch staging            # change the default

# One-off override for a single command:
klaviyo api /api/metrics/ --account prod
```

The key used for a request resolves in this order:

1. `--api-key` flag
2. `KLAVIYO_API_KEY` environment variable
3. The selected account's stored key, where the account is chosen by `--account` flag, then `KLAVIYO_ACCOUNT`, then the configured default

Profiles live in `~/.config/klaviyo/config.toml`; the API keys themselves are stored in the OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service). Where no keychain is available — headless Linux, containers — pass `--insecure-storage` to `auth login` to store the key in the config file instead, written with `0600` permissions. Keys stored in the file by older CLI versions keep working; run `klaviyo auth migrate` to move them into the keychain. In CI, skip stored accounts entirely and set `KLAVIYO_API_KEY`.

## Output and scripting

- **Terminals get tables, pipes get JSON.** List responses render as aligned tables on an interactive terminal; piped or redirected output is always pretty-printed JSON, so scripts never parse table text.
- **`--jq <expr>`** filters any response through a built-in jq interpreter ([gojq](https://github.com/itchyny/gojq)) — no jq install needed. Following jq convention, string results print raw and other values print as JSON, one result per line.
- **`--paginate`** follows `links.next` cursors and merges all pages' `data` into one response (GET list endpoints only). Combines with `--jq`, which runs on the merged result.
- **`--revision <date>`** overrides the pinned API revision header for a single call.
- **Errors:** non-2xx responses print the API's JSON:API error body and exit non-zero.

## Shell completion

Completion covers every command, flag, and configured account name:

```bash
# zsh (bash/fish/powershell also supported)
klaviyo completion zsh > "${fpath[1]}/_klaviyo"
```

## Environment variables

| Variable | Purpose |
| --- | --- |
| `KLAVIYO_API_KEY` | API key for requests, bypassing stored accounts (below `--api-key` in precedence) |
| `KLAVIYO_ACCOUNT` | Named account to use (below `--account` in precedence) |
| `KLAVIYO_CONFIG_DIR` | Config directory override (default `~/.config/klaviyo`) |
| `KLAVIYO_NO_UPDATE_NOTIFIER` | Disable the update check (also disabled when `CI` is set) |
| `VISUAL`, `EDITOR` | Editor for `klaviyo config -e` |
| `KLAVIYO_API_URL` | Base URL override for development and tests only; unsupported for normal use |

## Development

Requires Go 1.25+; everything else self-installs. See [ARCHITECTURE.md](ARCHITECTURE.md) for how the pieces fit together.

```bash
make build      # builds bin/klaviyo
make test       # go test -race ./...
make lint       # installs the pinned golangci-lint into ./bin, then runs it
make fmt        # gofumpt + goimports via golangci-lint fmt
```

CI runs lint, tests on all three platforms, a snapshot release build, and fails if `go mod tidy` would change the committed tree. The resource commands are generated from the vendored OpenAPI spec by an internal tool; regeneration happens via PRs when the spec updates.

Releases are cut by pushing a `v*` tag; GoReleaser builds and publishes the binaries.

Found a bug or want a feature? [Open an issue](https://github.com/klaviyo/klaviyo-cli/issues).

## License

[MIT](LICENSE)
