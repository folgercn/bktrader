import React from 'react';

export class ErrorBoundary extends React.Component<{ children: React.ReactNode }, { error: Error | null; componentStack: string }> {
  constructor(props: { children: React.ReactNode }) {
    super(props);
    this.state = { error: null, componentStack: "" };
  }

  static getDerivedStateFromError(error: Error) {
    return { error, componentStack: "" };
  }

  override componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error("console-render-error", error, info);
    this.setState({ componentStack: info.componentStack || "" });
  }

  override render() {
    if (this.state.error) {
      return (
        <div className="app-shell">
          <main className="main">
            <section className="panel">
              <div className="panel-header">
                <div>
                  <p className="panel-kicker">Render Error</p>
                  <h3>前端渲染失败</h3>
                </div>
              </div>
              <div className="modal-error">
                {this.state.error.message}
              </div>
              {this.state.error.stack ? (
                <pre className="error-stack">{this.state.error.stack}</pre>
              ) : null}
              {this.state.componentStack ? (
                <pre className="error-stack">{this.state.componentStack}</pre>
              ) : null}
            </section>
          </main>
        </div>
      );
    }
    return this.props.children;
  }
}
