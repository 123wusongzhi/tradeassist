import type { KeyboardEvent, ReactNode } from 'react';
import { ProCard, type ProCardProps } from '@ant-design/pro-components';
import { Typography } from 'antd';

export type MetricCardIntent = 'default' | 'primary' | 'success' | 'warning' | 'danger' | 'ai' | 'data';

export type MetricCardProps = Omit<ProCardProps, 'title'> & {
  title: ReactNode;
  value: ReactNode;
  description?: ReactNode;
  icon?: ReactNode;
  intent?: MetricCardIntent;
  onClick?: () => void;
};

export default function MetricCard({
  title,
  value,
  description,
  icon,
  intent = 'default',
  onClick,
  className,
  ...rest
}: MetricCardProps) {
  return (
    <ProCard
      {...rest}
      bordered
      hoverable={!!onClick}
      tabIndex={onClick ? 0 : undefined}
      role={onClick ? 'button' : undefined}
      onClick={onClick}
      onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => {
        if (!onClick) return;
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onClick();
        }
      }}
      className={['tm-metric-card', `tm-metric-card--${intent}`, onClick ? 'tm-metric-card--clickable' : '', className]
        .filter(Boolean)
        .join(' ')}
    >
      <div className="tm-metric-card__inner">
        <div className="tm-metric-card__content">
          <Typography.Text className="tm-metric-card__title">{title}</Typography.Text>
          <div className="tm-metric-card__value">{value}</div>
          {description ? <Typography.Text className="tm-metric-card__description">{description}</Typography.Text> : null}
        </div>
        {icon ? <span className="tm-metric-card__icon">{icon}</span> : null}
      </div>
    </ProCard>
  );
}
