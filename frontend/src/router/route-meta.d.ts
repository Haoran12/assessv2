import "vue-router";

declare module "vue-router" {
  interface RouteMeta {
    requiresAuth?: boolean;
    requiresRoot?: boolean;
    publicOnly?: boolean;
    allowWhenMustChange?: boolean;
    permission?: string | string[];
    useGlobalContext?: boolean;
  }
}
