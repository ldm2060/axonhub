# Remove Model Sharing & Visibility — Make Models Public by Default

## Problem

Model IDs are globally unique across users, so there is no conflict when different users create models. This makes the visibility/sharing system unnecessary for models — all models can be public by default.

Currently the model visibility system adds complexity: a `shared` state, a `shared_with` user list, share/unshare mutations, a "Shared with Me" page, and a publish request approval flow. None of this is needed for models.

## Design

### Scope

- **Models only.** Channel sharing/visibility/publishing remains unchanged.
- **DB fields preserved.** `visibility` and `shared_with` columns stay in the database; only logic and UI are removed.

### Frontend Changes

1. **Delete "Shared with Me" page entirely** — the whole page covers both channels and models, and the user wants it removed:
   - Delete `frontend/src/features/shared/` directory
   - Delete `frontend/src/routes/_authenticated/shared/` route
   - Remove "Shared with Me" entry from `sidebar.ts`

2. **Delete model share dialog**:
   - Delete `frontend/src/features/models/components/models-share-dialog.tsx`
   - Remove share dialog from `models-dialogs.tsx`
   - Remove "Share" action from model row actions (`data-table-row-actions.tsx`)

3. **Remove model sharing data layer**:
   - Remove `useShareModel`, `useUnshareModel`, `useMySharedModels` from `frontend/src/gql/sharing.ts` (keep channel share hooks)
   - Remove `visibility` and `sharedWith` from `frontend/src/features/models/data/schema.ts`
   - Remove model-related exports from `frontend/src/features/shared/data/shared.ts`

4. **Clean up i18n**: Remove model share/publish translation keys from `locales/` files.

### Backend Changes

1. **Model default visibility → published**: Change `visibility` field default in `internal/ent/schema/model.go` from `"private"` to `"published"`.

2. **Remove model sharing GraphQL**:
   - Remove `shareModel`, `unshareModel`, `mySharedModels` from `publish_request.graphql`
   - Remove corresponding resolvers from `publish_request.resolvers.go`

3. **Remove model sharing business logic**: Remove `ShareModel`, `UnshareModel`, model portion of `ListSharedWithUser` from `internal/server/biz/model.go`.

4. **Keep DB fields**: No migration — `visibility` and `shared_with` columns remain in the model table.

### What Stays Unchanged

- All channel sharing/visibility/publishing functionality
- Channel share dialog, channel share/unshare mutations
- Channel visibility query rules and privacy policies
- Publish request flow for channels
