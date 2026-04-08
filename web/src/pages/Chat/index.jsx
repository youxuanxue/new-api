/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useMemo } from 'react';
import { useTokenKeys } from '../../hooks/chat/useTokenKeys';
import { Button, Spin, Typography } from '@douyinfe/semi-ui';
import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { encodeToBase64 } from '../../helpers/base64';

/** Match token table "打开链接" substitution (SettingsChats templates). */
function resolveChatIntegrationUrl(template, serverAddress, rawKey) {
  if (!template || !serverAddress || !rawKey) return '';
  if (template.startsWith('ccswitch') || template.startsWith('fluent')) {
    return '__DESKTOP_FLOW__';
  }
  let url = template;
  const fullKey = `sk-${rawKey}`;
  if (url.includes('{cherryConfig}')) {
    const cherryConfig = {
      id: 'new-api',
      baseUrl: serverAddress,
      apiKey: fullKey,
    };
    url = url.replaceAll(
      '{cherryConfig}',
      encodeURIComponent(encodeToBase64(JSON.stringify(cherryConfig))),
    );
  } else if (url.includes('{aionuiConfig}')) {
    const aionuiConfig = {
      platform: 'new-api',
      baseUrl: serverAddress,
      apiKey: fullKey,
    };
    url = url.replaceAll(
      '{aionuiConfig}',
      encodeURIComponent(encodeToBase64(JSON.stringify(aionuiConfig))),
    );
  } else {
    url = url.replaceAll('{address}', encodeURIComponent(serverAddress));
    url = url.replaceAll('{key}', fullKey);
  }
  return url;
}

function readChatTemplate(chatIndex) {
  if (chatIndex === undefined || chatIndex === null || chatIndex === '') {
    return '';
  }
  const raw = localStorage.getItem('chats');
  if (!raw) return '';
  try {
    const chats = JSON.parse(raw);
    if (!Array.isArray(chats) || !chats[chatIndex]) return '';
    const entry = chats[chatIndex];
    for (const k in entry) {
      if (typeof entry[k] === 'string') return entry[k];
    }
  } catch {
    return '';
  }
  return '';
}

const ChatPage = () => {
  const { t } = useTranslation();
  const { id } = useParams();
  const { keys, serverAddress, isLoading } = useTokenKeys(id);

  const resolved = useMemo(() => {
    if (id == null || id === '') {
      return { kind: 'no_selection', url: '' };
    }
    if (keys.length === 0 || !serverAddress) {
      return { kind: 'loading', url: '' };
    }
    const template = readChatTemplate(id);
    if (!template) {
      return { kind: 'empty', url: '' };
    }
    const url = resolveChatIntegrationUrl(template, serverAddress, keys[0]);
    if (url === '__DESKTOP_FLOW__') {
      return { kind: 'desktop_flow', url: '' };
    }
    if (!url) {
      return { kind: 'empty', url: '' };
    }
    const isHttp = /^https?:\/\//i.test(url);
    return { kind: isHttp ? 'iframe' : 'deep_link', url };
  }, [id, keys, serverAddress]);

  const openDeepLink = () => {
    if (resolved.url) window.open(resolved.url, '_blank', 'noopener,noreferrer');
  };

  if (resolved.kind === 'no_selection') {
    return (
      <div
        className='flex flex-col items-center justify-center px-6'
        style={{ marginTop: '64px', minHeight: 'calc(100vh - 64px)' }}
      >
        <Typography.Paragraph type='secondary'>
          {t('请从左侧「聊天」菜单选择一个客户端入口。')}
        </Typography.Paragraph>
      </div>
    );
  }

  if (isLoading || resolved.kind === 'loading') {
    return (
      <div className='fixed inset-0 w-screen h-screen flex items-center justify-center bg-white/80 z-[1000] mt-[60px]'>
        <div className='flex flex-col items-center'>
          <Spin size='large' spinning={true} tip={null} />
          <span
            className='whitespace-nowrap mt-2 text-center'
            style={{ color: 'var(--semi-color-primary)' }}
          >
            {t('正在跳转...')}
          </span>
        </div>
      </div>
    );
  }

  if (resolved.kind === 'desktop_flow') {
    return (
      <div
        className='flex flex-col items-center justify-center px-6'
        style={{ marginTop: '64px', minHeight: 'calc(100vh - 64px)' }}
      >
        <Typography.Title heading={5}>
          {t('该聊天入口需在「令牌管理」中使用')}
        </Typography.Title>
        <Typography.Paragraph type='secondary' className='mt-2 text-center max-w-lg'>
          {t('流畅阅读 / CC Switch 类入口无法在页面内打开，请到令牌列表中点击对应「打开链接」按钮。')}
        </Typography.Paragraph>
        <Button
          theme='solid'
          className='mt-4'
          onClick={() => {
            window.location.href = '/console/token';
          }}
        >
          {t('前往令牌管理')}
        </Button>
      </div>
    );
  }

  if (resolved.kind === 'empty') {
    return (
      <div
        className='flex flex-col items-center justify-center px-6'
        style={{ marginTop: '64px', minHeight: 'calc(100vh - 64px)' }}
      >
        <Typography.Paragraph type='secondary'>
          {t('未找到聊天配置，请在系统设置 → 聊天中检查配置。')}
        </Typography.Paragraph>
      </div>
    );
  }

  if (resolved.kind === 'deep_link') {
    return (
      <div
        className='flex flex-col items-center justify-center px-6'
        style={{ marginTop: '64px', minHeight: 'calc(100vh - 64px)' }}
      >
        <Typography.Title heading={5}>
          {t('唤起本地聊天客户端')}
        </Typography.Title>
        <Typography.Paragraph type='secondary' className='mt-2 text-center max-w-lg'>
          {t(
            '此类入口（如 AionUI、Cherry Studio）使用自定义协议，无法在页面内嵌显示。请点击下方按钮；若已安装对应应用，系统会打开它。',
          )}
        </Typography.Paragraph>
        <Button theme='solid' className='mt-6' onClick={openDeepLink}>
          {t('打开应用 / 授权链接')}
        </Button>
      </div>
    );
  }

  return (
    <iframe
      src={resolved.url}
      style={{
        width: '100%',
        height: 'calc(100vh - 64px)',
        border: 'none',
        marginTop: '64px',
      }}
      title='Token Frame'
      allow='camera;microphone'
    />
  );
};

export default ChatPage;
