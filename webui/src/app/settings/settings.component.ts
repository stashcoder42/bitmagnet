import { Component, inject, signal } from "@angular/core";
import { Clipboard } from "@angular/cdk/clipboard";
import { MatSnackBar } from "@angular/material/snack-bar";
import { Apollo } from "apollo-angular";
import { AppModule } from "../app.module";
import { DocumentTitleComponent } from "../layout/document-title.component";
import * as generated from "../graphql/generated";

@Component({
  selector: "app-settings",
  templateUrl: "./settings.component.html",
  styleUrl: "./settings.component.scss",
  standalone: true,
  imports: [AppModule, DocumentTitleComponent],
})
export class SettingsComponent {
  private apollo = inject(Apollo);
  private clipboard = inject(Clipboard);
  private snackBar = inject(MatSnackBar);

  apiKeyInfo = signal<generated.ApiKeyInfo | null>(null);
  showApiKey = signal(false);
  loading = signal(true);
  rotating = signal(false);

  constructor() {
    this.loadApiKey();
  }

  private loadApiKey(): void {
    this.loading.set(true);
    this.apollo
      .query<generated.GetApiKeyQuery>({
        query: generated.GetApiKeyDocument,
        fetchPolicy: "no-cache",
      })
      .subscribe({
        next: (result) => {
          this.apiKeyInfo.set(result.data.apiKey.current);
          this.loading.set(false);
        },
        error: (error) => {
          console.error("Failed to load API key:", error);
          this.loading.set(false);
        },
      });
  }

  toggleShowApiKey(): void {
    this.showApiKey.update((v) => !v);
  }

  copyApiKey(): void {
    const key = this.apiKeyInfo()?.apiKey;
    if (key) {
      this.clipboard.copy(key);
      this.snackBar.open("API key copied to clipboard", "Close", {
        duration: 3000,
      });
    }
  }

  rotateApiKey(): void {
    if (
      !confirm(
        "Are you sure you want to rotate the API key? The current key will be invalidated and you will need to reload the page.",
      )
    ) {
      return;
    }

    this.rotating.set(true);
    this.apollo
      .mutate<generated.ApiKeyRotateMutation>({
        mutation: generated.ApiKeyRotateDocument,
        fetchPolicy: "no-cache",
      })
      .subscribe({
        next: () => {
          this.snackBar.open(
            "API key rotated successfully. Reloading page...",
            "Close",
            {
              duration: 3000,
            },
          );
          // Reload the page to get the new API key injected
          setTimeout(() => {
            window.location.reload();
          }, 1500);
        },
        error: (error) => {
          console.error("Failed to rotate API key:", error);
          this.snackBar.open("Failed to rotate API key", "Close", {
            duration: 5000,
          });
          this.rotating.set(false);
        },
      });
  }

  getMaskedApiKey(): string {
    const key = this.apiKeyInfo()?.apiKey;
    if (!key) return "";
    if (key.length <= 8) return "*".repeat(key.length);
    return key.substring(0, 4) + "*".repeat(key.length - 8) + key.substring(key.length - 4);
  }

  formatDate(dateString: string | undefined): string {
    if (!dateString) return "-";
    return new Date(dateString).toLocaleString();
  }
}
