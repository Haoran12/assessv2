import "vue-router";

declare module "vue-router" {
  interface RouteMeta {
    requiresAuth?: boolean;
    publicOnly?: boolean;
    allowWhenMustChange?: boolean;
    permission?: string | string[];
  }
}
