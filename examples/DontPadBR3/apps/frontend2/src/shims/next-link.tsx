import { Link as RouterLink, type LinkProps as RouterLinkProps } from "react-router-dom";

type NextLinkProps = Omit<RouterLinkProps, "to"> & {
  href: RouterLinkProps["to"];
};

export default function Link({ href, ...props }: NextLinkProps) {
  return <RouterLink to={href} {...props} />;
}
