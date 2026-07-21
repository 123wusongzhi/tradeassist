import type { CSSProperties, ReactNode } from 'react';
import { PageContainer, type PageContainerProps } from '@ant-design/pro-components';
import { layoutTokens } from '@/constants/layoutTokens';

export type TmPageContainerProps = PageContainerProps & {
  /** 页面内容最大宽度，默认 settings 1440 */
  contentMaxWidth?: number;
  /** 关闭内容区左右 padding，仅用于嵌入式特殊场景。 */
  padded?: boolean;
  /** 页面内容包裹层样式。 */
  contentWrapperStyle?: CSSProperties;
};

/**
 * 统一页面容器：标题 + 说明分行，内边距与最大宽度一致。
 */
export default function TmPageContainer({
  title,
  subTitle,
  contentMaxWidth = layoutTokens.settingsMaxWidth,
  padded = true,
  contentWrapperStyle,
  children,
  className,
  ...rest
}: TmPageContainerProps) {
  const wrapperStyle: CSSProperties = {
    maxWidth: contentMaxWidth,
    marginInline: 'auto',
    ...contentWrapperStyle,
  };

  return (
    <PageContainer
      {...rest}
      className={['tm-page-container', className].filter(Boolean).join(' ')}
      title={title}
      subTitle={subTitle}
    >
      <div
        className={['tm-page-container__content', padded ? '' : 'tm-page-container__content--unpadded']
          .filter(Boolean)
          .join(' ')}
        style={wrapperStyle}
      >
        {children}
      </div>
    </PageContainer>
  );
}

export function TmPageHeaderExtra({ children }: { children: ReactNode }) {
  return <div className="tm-page-header-extra">{children}</div>;
}
