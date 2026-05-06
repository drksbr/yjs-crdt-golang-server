import {
  type ComponentProps,
  type ComponentType,
  type LazyExoticComponent,
  Suspense,
  lazy,
} from "react";

type Loader<T> = () => Promise<{ default: T } | T>;

interface DynamicOptions {
  loading?: ComponentType;
  ssr?: boolean;
}

export default function dynamic<T extends ComponentType<any>>(
  loader: Loader<T>,
  options: DynamicOptions = {},
): ComponentType<ComponentProps<T>> {
  const LazyComponent = lazy(async () => {
    const loaded = await loader();
    if (typeof loaded === "function") {
      return { default: loaded as T };
    }
    return loaded as { default: T };
  }) as LazyExoticComponent<T>;

  const Loading = options.loading;

  return function DynamicComponent(props: ComponentProps<T>) {
    return (
      <Suspense fallback={Loading ? <Loading /> : null}>
        <LazyComponent {...props} />
      </Suspense>
    );
  };
}
