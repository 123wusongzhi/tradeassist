import { Drawer, type DrawerProps } from 'antd';
import { layoutTokens } from '@/constants/layoutTokens';

const DEFAULT_DRAWER_WIDTH = `min(${layoutTokens.drawerWidthMax}px, calc(100vw - ${layoutTokens.drawerViewportGap}px))`;

/** Project-wide drawer; explicit business widths still override this default. */
export default function AppDrawer({ width = DEFAULT_DRAWER_WIDTH, rootClassName, ...rest }: DrawerProps) {
  return (
    <Drawer
      width={width}
      rootClassName={['tm-app-drawer', rootClassName].filter(Boolean).join(' ')}
      {...rest}
    />
  );
}

export { DEFAULT_DRAWER_WIDTH };
