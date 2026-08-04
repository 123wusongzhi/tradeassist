import { history, useLocation } from "@umijs/max";
import { Spin } from "antd";
import { useEffect } from "react";

export default function LegacyOzonPublishRedirect() {
  const location = useLocation();
  useEffect(() => {
    history.replace(`/product/publishing-center${location.search || ""}`);
  }, [location.search]);
  return <Spin fullscreen tip="正在打开统一刊登中心…" />;
}
