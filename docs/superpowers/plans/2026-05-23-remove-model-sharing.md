# Remove Model Sharing & Visibility — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all model sharing/visibility logic and UI, delete the "Shared with Me" page, and make models public by default.

**Architecture:** Delete frontend components and data hooks for model sharing, remove the "Shared with Me" page entirely, strip model share/unshare GraphQL mutations and resolvers, remove model share biz methods, and change model visibility default to `published`.

**Tech Stack:** Go (Ent, gqlgen), React/TypeScript (TanStack Query, i18next)

---

### Task 1: Delete "Shared with Me" page and route

**Files:**
- Delete: `frontend/src/features/shared/` (entire directory)
- Delete: `frontend/src/routes/_authenticated/shared/index.tsx`

- [ ] **Step 1: Delete the shared features directory**

```bash
rm -rf frontend/src/features/shared
```

- [ ] **Step 2: Delete the shared route**

```bash
rm frontend/src/routes/_authenticated/shared/index.tsx && rmdir frontend/src/routes/_authenticated/shared
```

- [ ] **Step 3: Commit**

```bash
git add -A frontend/src/features/shared frontend/src/routes/_authenticated/shared
git commit -m "refactor: delete 'Shared with Me' page and route"
```

---

### Task 2: Remove "Shared with Me" sidebar entry and unused import

**Files:**
- Modify: `frontend/src/sidebar.ts:13` (IconShare import), `frontend/src/sidebar.ts:128-132` (sidebar entry)

- [ ] **Step 1: Remove the sidebar entry for "Shared with Me"**

In `frontend/src/sidebar.ts`, delete lines 128-132:

```ts
        {
          title: t('sidebar.items.sharedWithMe'),
          url: '/shared',
          icon: IconShare,
        } as NavLink,
```

- [ ] **Step 2: Remove the `IconShare` import if no longer used**

In `frontend/src/sidebar.ts`, remove `IconShare` from the import on line 13:

```ts
// Change this line:
  IconShare,
// Remove IconShare from the import list. If it was the only consumer, remove it entirely.
```

Check if `IconShare` is used elsewhere in the file. If not, remove it from the import.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/sidebar.ts
git commit -m "refactor: remove 'Shared with Me' sidebar entry"
```

---

### Task 3: Delete model share dialog and remove from dialogs/component tree

**Files:**
- Delete: `frontend/src/features/models/components/models-share-dialog.tsx`
- Modify: `frontend/src/features/models/components/models-dialogs.tsx:10,25-36`
- Modify: `frontend/src/features/models/context/models-context.tsx:16`

- [ ] **Step 1: Delete the model share dialog file**

```bash
rm frontend/src/features/models/components/models-share-dialog.tsx
```

- [ ] **Step 2: Remove share dialog import and render from models-dialogs.tsx**

In `frontend/src/features/models/components/models-dialogs.tsx`:

Remove the import on line 10:
```ts
import { ModelsShareDialog } from './models-share-dialog';
```

Remove the share dialog render block (lines 25-36):
```tsx
      {open === 'share' && currentRow && (
        <ModelsShareDialog
          open={true}
          onOpenChange={(isOpen) => {
            if (!isOpen) {
              setOpen(null);
              setCurrentRow(null);
            }
          }}
          model={currentRow}
        />
      )}
```

- [ ] **Step 3: Remove 'share' from DialogType union in models-context.tsx**

In `frontend/src/features/models/context/models-context.tsx`, remove `| 'share'` from the `DialogType` union (line 16):

```ts
// Change from:
  | 'share'
  | null;
// To:
  | null;
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/features/models/components/models-share-dialog.tsx frontend/src/features/models/components/models-dialogs.tsx frontend/src/features/models/context/models-context.tsx
git commit -m "refactor: delete model share dialog and remove from component tree"
```

---

### Task 4: Remove "Share" action from model row actions

**Files:**
- Modify: `frontend/src/features/models/components/data-table-row-actions.tsx:4,74-77`

- [ ] **Step 1: Remove IconShare import**

In `frontend/src/features/models/components/data-table-row-actions.tsx`, remove `IconShare` from the import on line 4:

```ts
// Change from:
import { IconEdit, IconArchive, IconArchiveOff, IconTrash, IconNote, IconShare } from '@tabler/icons-react';
// To:
import { IconEdit, IconArchive, IconArchiveOff, IconTrash, IconNote } from '@tabler/icons-react';
```

- [ ] **Step 2: Remove the Share dropdown menu item**

Remove lines 74-77:

```tsx
              <DropdownMenuItem onClick={() => openRowDialog('share')}>
                <IconShare size={16} className='mr-2' />
                {t('share.dialog.menuItem')}
              </DropdownMenuItem>
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/models/components/data-table-row-actions.tsx
git commit -m "refactor: remove 'Share' action from model row actions"
```

---

### Task 5: Remove model share/unshare hooks from sharing.ts

**Files:**
- Modify: `frontend/src/gql/sharing.ts:75-141`

- [ ] **Step 1: Remove model share/unshare mutation constants and hooks**

In `frontend/src/gql/sharing.ts`, delete lines 75-141 (the entire "Share/Unshare Model Mutations" section):

```ts
// Delete everything from "// Share/Unshare Model Mutations" through the end of useUnshareModel()
// This includes: SHARE_MODEL_MUTATION, UNSHARE_MODEL_MUTATION, useShareModel(), useUnshareModel()
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/gql/sharing.ts
git commit -m "refactor: remove model share/unshare hooks from sharing.ts"
```

---

### Task 6: Remove visibility and sharedWith from model schema

**Files:**
- Modify: `frontend/src/features/models/data/schema.ts:153-154`

- [ ] **Step 1: Remove visibility and sharedWith from modelSchema**

In `frontend/src/features/models/data/schema.ts`, remove lines 153-154 from the `modelSchema` definition:

```ts
// Remove these two lines:
  visibility: z.enum(['private', 'shared', 'published']).default('private'),
  sharedWith: z.array(z.number()).optional().default([]).nullable(),
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/features/models/data/schema.ts
git commit -m "refactor: remove visibility and sharedWith from model schema"
```

---

### Task 7: Remove i18n keys for model sharing

**Files:**
- Modify: `frontend/src/locales/en/sharing.json:4,14`
- Modify: `frontend/src/locales/zh-CN/sharing.json:4,14`
- Modify: `frontend/src/locales/en/base.json:23,359`
- Modify: `frontend/src/locales/zh-CN/base.json:23,359`

- [ ] **Step 1: Remove model-specific sharing i18n keys**

In `frontend/src/locales/en/sharing.json`, remove:
```json
  "share.dialog.description.model": "Manage sharing and visibility for this model.",
```
```json
  "share.dialog.requestPublishDescription.model": "Request to publish model \"{{name}}\". This will make it visible to all users after approval.",
```

In `frontend/src/locales/zh-CN/sharing.json`, remove:
```json
  "share.dialog.description.model": "管理此模型的共享和可见性。",
```
```json
  "share.dialog.requestPublishDescription.model": "申请发布模型\"{{name}}\"。审批通过后，所有用户将可见。",
```

- [ ] **Step 2: Remove "Shared with Me" sidebar and page i18n keys from base.json**

In `frontend/src/locales/en/base.json`, remove:
```json
  "sidebar.items.sharedWithMe": "Shared with Me",
```
```json
  "shared.title": "Shared with Me",
```

In `frontend/src/locales/zh-CN/base.json`, remove:
```json
  "sidebar.items.sharedWithMe": "与我共享",
```
```json
  "shared.title": "与我共享",
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/locales/
git commit -m "refactor: remove model sharing and 'Shared with Me' i18n keys"
```

---

### Task 8: Change model visibility default to published

**Files:**
- Modify: `internal/ent/schema/model.go:59`

- [ ] **Step 1: Change default value**

In `internal/ent/schema/model.go`, change line 59:

```go
// Change from:
field.Enum("visibility").Values("private", "shared", "published").Default("private").Annotations(entgql.OrderField("VISIBILITY")),
// To:
field.Enum("visibility").Values("private", "shared", "published").Default("published").Annotations(entgql.OrderField("VISIBILITY")),
```

- [ ] **Step 2: Regenerate Ent code**

```bash
cd internal/ent && go generate ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/ent/schema/model.go internal/ent/
git commit -m "feat: change model visibility default to published"
```

---

### Task 9: Remove model share/unshare/mySharedModels from GraphQL schema

**Files:**
- Modify: `internal/server/gql/publish_request.graphql:7-8,13`

- [ ] **Step 1: Remove model share/unshare mutations and query from GraphQL schema**

In `internal/server/gql/publish_request.graphql`, remove:

Line 7: `  shareModel(id: ID!, userIDs: [ID!]!): Model!`
Line 8: `  unshareModel(id: ID!, userIDs: [ID!]!): Model!`
Line 13: `  mySharedModels: [Model!]!`

The file should become:
```graphql
extend type Mutation {
  requestPublish(resourceType: PublishRequestResourceType!, resourceID: ID!, comment: String): PublishRequest!
  cancelPublishRequest(id: ID!): Boolean!
  reviewPublishRequest(id: ID!, action: ReviewAction!, comment: String): PublishRequest!
  shareChannel(id: ID!, userIDs: [ID!]!): Channel!
  unshareChannel(id: ID!, userIDs: [ID!]!): Channel!
}

extend type Query {
  mySharedChannels: [Channel!]!
  myDashboard: DashboardOverview!
}

enum ReviewAction {
  approve
  reject
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/gql/publish_request.graphql
git commit -m "refactor: remove model share/unshare/mySharedModels from GraphQL schema"
```

---

### Task 10: Remove model share/unshare/mySharedModels resolvers

**Files:**
- Modify: `internal/server/gql/publish_request.resolvers.go:75-111`

- [ ] **Step 1: Remove the three resolver functions**

In `internal/server/gql/publish_request.resolvers.go`, delete:

- `ShareModel` resolver (lines 75-83)
- `UnshareModel` resolver (lines 85-93)
- `MySharedModels` resolver (lines 104-111)

- [ ] **Step 2: Regenerate gqlgen code**

```bash
cd internal/server/gql && go generate ./...
```

This will update `generated.go` to remove the model share/unshare/mySharedModels entries.

- [ ] **Step 3: Commit**

```bash
git add internal/server/gql/
git commit -m "refactor: remove model share/unshare/mySharedModels resolvers"
```

---

### Task 11: Remove model share/unshare/ListSharedWithUser from biz service

**Files:**
- Modify: `internal/server/biz/model.go:862-935`

- [ ] **Step 1: Remove the three biz methods**

In `internal/server/biz/model.go`, delete:

- `ShareModel` method (lines 862-891)
- `UnshareModel` method (lines 893-921)
- `ListSharedWithUser` method (lines 923-935)

- [ ] **Step 2: Verify the code compiles**

```bash
go build ./internal/server/biz/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/biz/model.go
git commit -m "refactor: remove model ShareModel/UnshareModel/ListSharedWithUser from biz"
```

---

### Task 12: Final build verification

- [ ] **Step 1: Run Go build for the entire project**

```bash
go build ./...
```

- [ ] **Step 2: Run frontend type check**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit any remaining generated changes**

```bash
git add -A
git commit -m "chore: update generated code after removing model sharing"
```
