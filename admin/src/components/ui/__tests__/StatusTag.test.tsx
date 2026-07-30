import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import StatusTag from '../StatusTag';

describe('StatusTag', () => {
  it('renders shared collect task labels for known statuses', () => {
    render(<StatusTag status="running" />);

    expect(screen.getByText('处理中')).toBeInTheDocument();
  });

  it('prefers explicit text and className overrides', () => {
    render(<StatusTag status="failed" text="平台返回失败" className="custom-status" />);

    const tag = screen.getByText('平台返回失败');
    expect(tag).toBeInTheDocument();
    expect(tag.closest('.custom-status')).not.toBeNull();
  });
});
