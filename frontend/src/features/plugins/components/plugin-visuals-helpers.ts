// 职责: 插件列表的搜索匹配，与视觉组件分开以保住 fast refresh。
// 关联文件:
//   frontend/src/features/plugins/components/plugin-visuals.tsx
//   frontend/src/features/plugins/index.tsx

export function matchesPluginSearch(search: string, ...fields: (string | null | undefined)[]) {
  const query = search.trim().toLowerCase();
  if (!query) {
    return true;
  }
  return fields.some((field) => field?.toLowerCase().includes(query));
}
