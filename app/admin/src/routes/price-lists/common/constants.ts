/**
 * Re-implementation of the enum (kept local to avoid a cross-package import)
 */
export enum PriceListStatus {
  ACTIVE = "active",
  DRAFT = "draft",
}

export enum PriceListDateStatus {
  SCHEDULED = "scheduled",
  EXPIRED = "expired",
}

/**
 * Re-implementation of the enum (kept local to avoid a cross-package import)
 */
export enum PriceListType {
  SALE = "sale",
  OVERRIDE = "override",
}
