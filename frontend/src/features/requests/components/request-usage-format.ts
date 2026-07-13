export function formatRequestUsageCost(
  cost: number | null | undefined,
  currencyCode: string | undefined,
  format: (value: number, currencyCode: string) => string
): string {
  if (!cost || cost <= 0 || !currencyCode) return '-';
  return format(cost, currencyCode);
}
