import { history } from '@umijs/max';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import EmptyState from '../EmptyState';

const historyMock = vi.mocked(history);

describe('EmptyState', () => {
  it('renders title, description and primary action', () => {
    render(<EmptyState title="暂无商品" description="先采集或手动创建商品" actionLabel="创建商品" />);

    expect(screen.getByText('暂无商品')).toBeInTheDocument();
    expect(screen.getByText('先采集或手动创建商品')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '创建商品' })).toBeInTheDocument();
  });

  it('prefers custom action callbacks over route navigation', async () => {
    const onAction = vi.fn();
    render(<EmptyState title="暂无商品" actionLabel="创建商品" actionPath="/products/new" onAction={onAction} />);

    await userEvent.click(screen.getByRole('button', { name: '创建商品' }));

    expect(onAction).toHaveBeenCalledTimes(1);
    expect(historyMock.push).not.toHaveBeenCalled();
  });

  it('navigates to actionPath when no callback is provided', async () => {
    render(<EmptyState title="暂无商品" actionLabel="创建商品" actionPath="/products/new" />);

    await userEvent.click(screen.getByRole('button', { name: '创建商品' }));

    expect(historyMock.push).toHaveBeenCalledWith('/products/new');
  });
});
